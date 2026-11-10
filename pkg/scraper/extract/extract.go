package extract

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

// AdminReportsByURL fetches one House Administration quarterly report page.
func (s *ExtractSvc) AdminReportsByURL(reportURL string) (*http.Response, error) {
	return s.rm.Get(reportURL)
}

// GetByURL fetches a known House of Commons source URL.
func (s *ExtractSvc) GetByURL(sourceURL string) (*http.Response, error) {
	return s.rm.Get(sourceURL)
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
func (s *ExtractSvc) MpProfile(name types.PersonName) (*http.Response, error) {
	searchText := url.QueryEscape(fmt.Sprintf("%s %s", name.FirstName, name.LastName))
	url := fmt.Sprintf("https://www.ourcommons.ca/Members/en/search?searchText=%s&parliament=all", searchText)

	resp, err := s.rm.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// MpProfileByURL fetches an exact MP profile URL discovered from the member directory.
func (s *ExtractSvc) MpProfileByURL(profileURL string) (*http.Response, error) {
	return s.rm.Get(profileURL)
}

// Returns raw Members of Parliament spending data
func (s *ExtractSvc) AllMpExpenses(h types.ExpenseURLs) (*types.RawExpenseResponses, error) {
	var (
		wg          sync.WaitGroup
		mu          sync.Mutex
		contract    *http.Response
		hospitality *http.Response
		travel      *http.Response
		errs        []error
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		if strings.TrimSpace(h.Contract) == "" {
			return
		}

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
		if strings.TrimSpace(h.Hospitality) == "" {
			return
		}

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
		if strings.TrimSpace(h.Travel) == "" {
			return
		}

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
		for _, response := range []*http.Response{contract, hospitality, travel} {
			if response != nil && response.Body != nil {
				_ = response.Body.Close()
			}
		}
		return nil, fmt.Errorf("[EXTRACTION FAILED] %v", errs)
	}

	return &types.RawExpenseResponses{
		Contract:    contract,
		Hospitality: hospitality,
		Travel:      travel,
	}, nil
}
