package extract

import (
	"fmt"
	"net/http"
	"time"
)

func (s *ExtractionSvc) Get(url string) (*http.Response, error) {
	const maxRetries = 5
	timeout := time.Second * 3

	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 200 {
			return resp, nil
		}
		timeout += time.Second
	}

	return nil, fmt.Errorf("[RATE LIMITED] Request failed.\tURL: %s", url)
}
