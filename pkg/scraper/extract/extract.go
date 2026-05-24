package extract

import (
	"fmt"
	"net/http"
	"net/url"

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

func (s *ExtractSvc) AdminReports() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/proactivedisclosure/en/house-administration"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *ExtractSvc) OfficerReports() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/proactivedisclosure/en/house-officers"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (s *ExtractSvc) MpReports() (*http.Response, error) {
	const URL = "https://www.ourcommons.ca/proactivedisclosure/en/members"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *ExtractSvc) MpProfileUrl() (*http.Response, error) {
const URL = "https://www.ourcommons.ca/Members/en/search"

	resp, err := s.rm.Get(URL)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *ExtractSvc) MpProfile(name types.Name) (*http.Response, error) {
searchText := url.QueryEscape(fmt.Sprintf("%s %s", name.FirstName, name.LastName))
	url := fmt.Sprintf("https://www.ourcommons.ca/Members/en/search?searchText=%s&parliament=all", searchText)

	resp, err := s.rm.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}