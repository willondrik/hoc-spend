package requests

import (
	"fmt"
	"net/http"
	"time"
)

const (
	defaultTimeout = 30 * time.Second
	maxRetries     = 5
)

// RequestManager performs HTTP requests for the scraper.
type RequestManager struct {
	client     *http.Client
	retryDelay func(time.Duration)
}

// NewRequestManager creates a request manager with a bounded request timeout.
func NewRequestManager() *RequestManager {
	return NewRequestManagerWithClient(&http.Client{Timeout: defaultTimeout})
}

// NewRequestManagerWithClient creates a request manager using client. It is
// useful when an application needs a custom transport and for deterministic
// HTTP tests. A nil client receives the standard bounded-timeout client.
func NewRequestManagerWithClient(client *http.Client) *RequestManager {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	return &RequestManager{client: client, retryDelay: time.Sleep}
}

// SetRetryDelay overrides retry waiting. It is primarily intended for fast,
// deterministic tests; passing nil restores the standard delay behavior.
func (r *RequestManager) SetRetryDelay(delay func(time.Duration)) {
	r.retryDelay = delay
}

// Get fetches a URL and retries retryable non-success responses. The caller owns
// the returned response body and must close it when finished reading it.
func (r *RequestManager) Get(url string) (*http.Response, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := r.client.Get(url)
		if err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("request %q: failed after %d attempts: %w", url, maxRetries, err)
			}
			r.wait(attempt)
			continue
		}

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return resp, nil
		}

		resp.Body.Close()
		if resp.StatusCode < http.StatusInternalServerError && resp.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("request %q: unexpected status %s", url, resp.Status)
		}

		if attempt < maxRetries-1 {
			r.wait(attempt)
		}
	}

	return nil, fmt.Errorf("request %q: failed after %d attempts", url, maxRetries)
}

func (r *RequestManager) wait(attempt int) {
	delay := r.retryDelay
	if delay == nil {
		delay = time.Sleep
	}
	delay(time.Duration(attempt+1) * time.Second)
}
