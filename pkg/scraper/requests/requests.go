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
	client *http.Client
}

// NewRequestManager creates a request manager with a bounded request timeout.
func NewRequestManager() *RequestManager {
	return &RequestManager{
		client: &http.Client{Timeout: defaultTimeout},
	}
}

// Get fetches a URL and retries retryable non-success responses. The caller owns
// the returned response body and must close it when finished reading it.
func (r *RequestManager) Get(url string) (*http.Response, error) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err := r.client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("request %q: %w", url, err)
		}

		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
			return resp, nil
		}

		resp.Body.Close()
		if resp.StatusCode < http.StatusInternalServerError && resp.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("request %q: unexpected status %s", url, resp.Status)
		}

		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("request %q: failed after %d attempts", url, maxRetries)
}
