package extract

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willondrik/hoc-spend/pkg/scraper/requests"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestAllMpExpensesClosesSuccessfulResponsesWhenAnotherCategoryFails(t *testing.T) {
	var closed atomic.Int32
	manager := requests.NewRequestManagerWithClient(&http.Client{Transport: testRoundTripper(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/travel" {
			return nil, errors.New("network failure")
		}
		return &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       closingBody{ReadCloser: io.NopCloser(strings.NewReader("ok")), closed: &closed},
		}, nil
	})})
	manager.SetRetryDelay(func(time.Duration) {})

	service := NewSvc(manager)
	_, err := service.AllMpExpenses(types.ExpenseURLs{
		Contract:    "https://example.test/contracts",
		Hospitality: "https://example.test/hospitality",
		Travel:      "https://example.test/travel",
	})
	if err == nil {
		t.Fatal("AllMpExpenses() error = nil, want error")
	}
	if closed.Load() != 2 {
		t.Fatalf("closed successful response bodies = %d, want 2", closed.Load())
	}
}

func TestAllMpExpensesSkipsEmptyCategoryURLs(t *testing.T) {
	var requestCount atomic.Int32
	manager := requests.NewRequestManagerWithClient(&http.Client{Transport: testRoundTripper(func(*http.Request) (*http.Response, error) {
		requestCount.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader("ok")), Header: make(http.Header)}, nil
	})})
	service := NewSvc(manager)

	responses, err := service.AllMpExpenses(types.ExpenseURLs{Travel: "https://example.test/travel"})
	if err != nil {
		t.Fatalf("AllMpExpenses() error = %v", err)
	}
	defer responses.Travel.Body.Close()
	if responses.Contract != nil || responses.Hospitality != nil || responses.Travel == nil {
		t.Fatalf("unexpected responses: %#v", responses)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requestCount.Load())
	}
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (fn testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type closingBody struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (body closingBody) Close() error {
	body.closed.Add(1)
	return body.ReadCloser.Close()
}
