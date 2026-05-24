package requests

import (
	"fmt"
	"net/http"
)

type RequestManager struct {
	client *http.Client
}

func NewRequestManager() *RequestManager {
	return &RequestManager{
		client: &http.Client{},
	}
}

func (r *RequestManager) Get(url string) (*http.Response, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("[REQUEST FAILED] %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		panic("rate limited")

		return nil, fmt.Errorf("[REQUEST FAILED] %d error", resp.StatusCode)
	}

	return resp, nil
}


