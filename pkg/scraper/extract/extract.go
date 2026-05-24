package extract

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/willondrik/hoc-spend/pkg/scraper/requests"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

type ExtractSvc struct {
	rm *requests.RequestManager
} 

func NewSvc(rm *requests.RequestManager) *ExtractSvc {
	return &ExtractSvc{
		rm: rm,
	}
}

// Returns admin expenditure report data
func (s *ExtractSvc) AdminReports() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/proactivedisclosure/en/house-administration"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Returns officer expenditure report data
func (s *ExtractSvc) OfficerReports() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/proactivedisclosure/en/house-officers"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}

	return resp, err
}

// Returns Members of Parliament expenditure report data
func (s *ExtractSvc) MpReports() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/proactivedisclosure/en/members"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Returns a Member of Parliament's profile URL
func (s *ExtractSvc) MpProfileUrl() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/Members/en/search"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Returns a Member of Parliament's profile data
func (s *ExtractSvc) MpProfile(name types.Name) (*http.Response, error) {
	searchText := url.QueryEscape(fmt.Sprintf("%s %s", name.FirstName, name.LastName))
	url := fmt.Sprintf("https://www.ourcommons.ca/Members/en/search?searchText=%s&parliament=all", searchText)

	resp, err := s.rm.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Returns raw Members of Parliament spending data
func (s *ExtractSvc) AllMpExpenses(h types.Hrefs) (*types.RawExpenses, error) {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		contract *http.Response
		hospitality *http.Response
		travel *http.Response
		errs []error
	)

	wg.Add(3)
	go func() {
		defer wg.Done()

		expenses, err := s.rm.Get(h.Contract)
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}
		contract = expenses
	}()

	go func() {
		defer wg.Done()

		expenses, err := s.rm.Get(h.Hospitality)
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}
		hospitality = expenses
	}()


	go func() {
		defer wg.Done()

		expenses, err := s.rm.Get(h.Travel)
		if err != nil {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			return
		}
		travel = expenses
	}()

	wg.Wait()

	if len(errs) != 0 {
		return nil, fmt.Errorf("[EXTRACTION FAILED] %v", errs)
	}

	return &types.RawExpenses{
		Contract: contract,
		Hospitality: hospitality,
		Travel: travel,
	}, nil
}