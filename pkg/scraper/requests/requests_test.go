package requests

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetRetriesRetryableStatusAndClosesDiscardedBodies(t *testing.T) {
	var attempts atomic.Int32
	var closed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("retry"))
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	manager := NewRequestManagerWithClient(&http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		response, err := http.DefaultTransport.RoundTrip(request)
		if err != nil || response.StatusCode < http.StatusInternalServerError {
			return response, err
		}
		response.Body = closeCounter{ReadCloser: response.Body, closed: &closed}
		return response, nil
	})})
	manager.retryDelay = func(_ time.Duration) {}

	response, err := manager.Get(server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer response.Body.Close()
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
	if closed.Load() != 2 {
		t.Fatalf("discarded response bodies closed = %d, want 2", closed.Load())
	}
}

func TestGetDoesNotRetryNonRetryableStatus(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	manager := NewRequestManagerWithClient(server.Client())
	manager.retryDelay = func(_ time.Duration) { t.Fatal("non-retryable status slept") }
	if _, err := manager.Get(server.URL); err == nil {
		t.Fatal("Get() error = nil, want 404 error")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}

func TestGetRetriesTransientTransportErrors(t *testing.T) {
	var attempts atomic.Int32
	manager := NewRequestManagerWithClient(&http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
		if attempts.Add(1) < 3 {
			return nil, errors.New("temporary transport failure")
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})})
	manager.retryDelay = func(_ time.Duration) {}

	response, err := manager.Get("https://example.test/retry")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer response.Body.Close()
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type closeCounter struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (body closeCounter) Close() error {
	body.closed.Add(1)
	return body.ReadCloser.Close()
}
