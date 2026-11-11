package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/willondrik/hoc-spend/pkg/scraper/extract"
	"github.com/willondrik/hoc-spend/pkg/scraper/requests"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestAdministrationReportFetchesAndParsesAllCategoriesWithRetry(t *testing.T) {
	var contractAttempts atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/report":
			_, _ = w.Write([]byte(`<a class="view-report-link" href="` + serverURL(request) + `/travel">travel</a><a class="view-report-link" href="` + serverURL(request) + `/hospitality">hospitality</a><a class="view-report-link" href="` + serverURL(request) + `/contracts">contracts</a>`))
		case "/contracts":
			if contractAttempts.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`<a class="csv-btn view-report-link" href="/downloads/contracts.csv">CSV</a><table class="table contracts table-striped" summary="Contracts"><tbody><tr><td>Supplier</td><td>Description</td><td>From April 1, 2026 to June 30, 2026</td><td>$10.00</td><td>$11.00</td></tr></tbody></table>`))
		case "/hospitality":
			_, _ = w.Write([]byte(`<a class="csv-btn view-report-link" href="/downloads/hospitality.csv">CSV</a><table class="table expenses table-striped" summary="Hospitality Events"><tbody><tr><td>E-1</td><td>Host</td><td>Purpose</td><td>April 1, 2026</td><td>$12.00</td></tr><tr><td><p aria-labelledby="TotalAttendees">2</p></td></tr></tbody></table>`))
		case "/travel":
			_, _ = w.Write([]byte(`<a class="csv-btn view-report-link" href="/downloads/travel.csv">CSV</a><table class="table expenses table-striped" summary="Travel Events"><tbody><tr><td>T-1</td><td>Jane Doe</td><td>Purpose</td><td>From April 1, 2026 to April 2, 2026</td><td>$13.00</td></tr><tr><td><p aria-labelledby="PlacesVisited">Ottawa</p><p aria-labelledby="Transportation">$4.00</p><p aria-labelledby="Accommodations">$3.00</p><p aria-labelledby="MealsAndIncidentals">$2.00</p><p aria-labelledby="Other">$4.00</p></td></tr></tbody></table>`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	service := testService(server.Client())
	report, err := administrationReport(service, types.ExpenditureReport{URL: server.URL + "/report", FiscalYear: 2026, FiscalQuarter: 1})
	if err != nil {
		t.Fatalf("administrationReport() error = %v", err)
	}
	if contractAttempts.Load() != 2 {
		t.Fatalf("contract attempts = %d, want retry to make 2", contractAttempts.Load())
	}
	if len(report.Contract.Entries) != 1 || len(report.Hospitality.Entries) != 1 || len(report.Travel.Entries) != 1 {
		t.Fatalf("parsed category counts = contracts:%d hospitality:%d travel:%d, want one each", len(report.Contract.Entries), len(report.Hospitality.Entries), len(report.Travel.Entries))
	}
	if report.Contract.CSVURL != "https://www.ourcommons.ca/downloads/contracts.csv" || report.Hospitality.CSVURL != "https://www.ourcommons.ca/downloads/hospitality.csv" || report.Travel.CSVURL != "https://www.ourcommons.ca/downloads/travel.csv" {
		t.Fatalf("CSV URLs = contract:%q hospitality:%q travel:%q, want House of Commons CSV download URLs", report.Contract.CSVURL, report.Hospitality.CSVURL, report.Travel.CSVURL)
	}
}

func TestOfficerExpensesAllowsZeroSpendCategoryWithoutURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/contracts":
			_, _ = w.Write([]byte(`<table><tbody><tr><td>Acme Corp.</td><td>Description</td><td>April 1, 2026</td><td>$10.00</td></tr></tbody></table>`))
		case "/hospitality":
			_, _ = w.Write([]byte(`<table><tbody><tr><td>April 1, 2026</td><td>Ottawa</td><td>2</td><td>Purpose</td><td>$8.00</td></tr><tr><td><table><tbody><tr><td>Supplier</td><td>$8.00</td></tr></tbody></table></td></tr></tbody></table>`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	expenses, err := officerExpenses(testService(server.Client()), types.OfficerExpenseSummary{
		ContractExpenses:    types.ExpenseCategory{URL: server.URL + "/contracts"},
		HospitalityExpenses: types.ExpenseCategory{URL: server.URL + "/hospitality"},
		TravelExpenses:      types.ExpenseCategory{},
	})
	if err != nil {
		t.Fatalf("officerExpenses() error = %v", err)
	}
	if len(expenses.ContractExpenses) != 1 || len(expenses.HospitalityExpenses) != 1 || len(expenses.TravelExpenses) != 0 {
		t.Fatalf("parsed category counts = contracts:%d hospitality:%d travel:%d, want 1,1,0", len(expenses.ContractExpenses), len(expenses.HospitalityExpenses), len(expenses.TravelExpenses))
	}
}

func TestMPReportSubjectsFetchesAllDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/report":
			_, _ = w.Write([]byte(`<table id="data-table"><tbody><tr><td>Doe, Jane</td><td>Example Riding</td><td>Example Caucus</td><td>$100.00</td><td><a href="` + serverURL(request) + `/travel">$30.00</a></td><td><a href="` + serverURL(request) + `/hospitality">$20.00</a></td><td><a href="` + serverURL(request) + `/contracts">$10.00</a></td></tr></tbody></table>`))
		case "/contracts":
			_, _ = w.Write([]byte(`<table><tbody><tr><td>Acme Corp.</td><td>Description</td><td>April 1, 2026</td><td>$10.00</td></tr></tbody></table>`))
		case "/hospitality":
			_, _ = w.Write([]byte(`<table><tbody><tr class="expenses-main-info"><td>April 1, 2026</td><td>Ottawa</td><td>2</td><td>Purpose</td><td>$20.00</td></tr><tr><td><div class="col-md-4">Reception</div><table><tbody></tbody></table></td></tr></tbody></table>`))
		case "/travel":
			_, _ = w.Write([]byte(`<table><tbody><tr class="expenses-main-info"><td>April 1, 2026</td><td>Ottawa</td><td>Toronto</td><td>$10.00</td><td>$5.00</td><td>$5.00</td><td>$30.00</td><td></td></tr></tbody></table>`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	report := types.ExpenditureReport{URL: server.URL + "/report", FiscalYear: 2026, FiscalQuarter: 1, DateRange: types.DateRange{StartDate: "2026-04-01", EndDate: "2026-06-30"}}
	subjects, profiles, err := mpReportSubjects(testService(server.Client()), report)
	if err != nil {
		t.Fatalf("mpReportSubjects() error = %v", err)
	}
	if len(subjects) != 1 || len(profiles) != 1 {
		t.Fatalf("subjects=%d profiles=%d, want 1 each", len(subjects), len(profiles))
	}
	subject := subjects[0]
	if subject.SourceIdentity != profiles[0].SourceIdentity || !strings.HasPrefix(subject.SourceIdentity, "source://ourcommons.ca/proactivedisclosure/mp/") {
		t.Fatalf("unexpected source identity: subject=%q profile=%q", subject.SourceIdentity, profiles[0].SourceIdentity)
	}
	if len(subject.Expenses.ContractExpenses) != 1 || len(subject.Expenses.HospitalityExpenses) != 1 || len(subject.Expenses.TravelExpenses) != 1 {
		t.Fatalf("detail counts = contracts:%d hospitality:%d travel:%d, want 1 each", len(subject.Expenses.ContractExpenses), len(subject.Expenses.HospitalityExpenses), len(subject.Expenses.TravelExpenses))
	}
}

func testService(client *http.Client) *extract.ExtractSvc {
	manager := requests.NewRequestManagerWithClient(client)
	manager.SetRetryDelay(func(time.Duration) {})
	return extract.NewSvc(manager)
}

func serverURL(request *http.Request) string {
	return "http://" + request.Host
}
