// Command scraper discovers House of Commons expenditure reports and compares
// them with the configured PostgreSQL database.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/willondrik/hoc-spend/db"
	"github.com/willondrik/hoc-spend/pkg/scraper/extract"
	"github.com/willondrik/hoc-spend/pkg/scraper/load"
	"github.com/willondrik/hoc-spend/pkg/scraper/requests"
	"github.com/willondrik/hoc-spend/pkg/scraper/transform"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

type discovery struct {
	name   string
	domain load.ReportDomain
	fetch  func() (*http.Response, error)
	parse  func(*http.Response) (types.ExpenditureReports, error)
}

type runOptions struct {
	domain        string
	apply         bool
	limit         int
	fiscalYear    int
	fiscalQuarter int
}

func main() {
	var (
		domain        = flag.String("domain", "all", "report domain to inspect: all, mp, administration, or officer")
		apply         = flag.Bool("apply", false, "fetch, parse, and load new reports")
		limit         = flag.Int("limit", 1, "maximum new reports to load with -apply; zero loads none")
		fiscalYear    = flag.Int("fiscal-year", 0, "optional fiscal year filter; requires -fiscal-quarter")
		fiscalQuarter = flag.Int("fiscal-quarter", 0, "optional fiscal quarter filter (1-4); requires -fiscal-year")
	)
	flag.Parse()

	opts := runOptions{
		domain:        *domain,
		apply:         *apply,
		limit:         *limit,
		fiscalYear:    *fiscalYear,
		fiscalQuarter: *fiscalQuarter,
	}
	if err := opts.validate(); err != nil {
		log.Fatal(err)
	}

	timeout := 2 * time.Minute
	if opts.apply {
		timeout = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	database, err := db.NewContext(ctx)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer database.Close()

	loader, err := load.New(database.Pool)
	if err != nil {
		log.Fatalf("create loader: %v", err)
	}
	service := extract.NewSvc(requests.NewRequestManager())

	checks := []discovery{
		{name: "MP", domain: load.ReportDomainMP, fetch: service.MpReports, parse: transform.MpExpenditureReports},
		{name: "Administration", domain: load.ReportDomainAdministration, fetch: service.AdminReports, parse: transform.AdministratorExpenditureReports},
		{name: "Officer", domain: load.ReportDomainOfficer, fetch: service.OfficerReports, parse: transform.OfficerExpenditureReports},
	}

	selected, err := selectDiscoveries(*domain, checks)
	if err != nil {
		log.Fatal(err)
	}
	for _, check := range selected {
		if err := runDomain(ctx, loader, service, check, opts); err != nil {
			log.Fatal(err)
		}
	}
}

func (opts runOptions) validate() error {
	if opts.limit < 0 {
		return fmt.Errorf("-limit must be zero or greater")
	}
	if (opts.fiscalYear == 0) != (opts.fiscalQuarter == 0) {
		return fmt.Errorf("-fiscal-year and -fiscal-quarter must be supplied together")
	}
	if opts.fiscalYear != 0 && (opts.fiscalYear < 1867 || opts.fiscalQuarter < 1 || opts.fiscalQuarter > 4) {
		return fmt.Errorf("invalid fiscal period FY%d Q%d", opts.fiscalYear, opts.fiscalQuarter)
	}
	return nil
}

func selectDiscoveries(value string, checks []discovery) ([]discovery, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "all" {
		return checks, nil
	}
	for _, check := range checks {
		if value == string(check.domain) {
			return []discovery{check}, nil
		}
	}
	return nil, fmt.Errorf("unsupported -domain %q; use all, mp, administration, or officer", value)
}

func runDomain(ctx context.Context, loader *load.Loader, service *extract.ExtractSvc, check discovery, opts runOptions) error {
	response, err := check.fetch()
	if err != nil {
		return fmt.Errorf("fetch %s report index: %w", check.name, err)
	}
	reports, parseErr := check.parse(response)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return fmt.Errorf("parse %s report index: %w", check.name, parseErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s report index response: %w", check.name, closeErr)
	}

	newReports, err := loader.NewReports(ctx, check.domain, reports.Reports)
	if err != nil {
		return fmt.Errorf("compare %s reports with database: %w", check.name, err)
	}

	newReports = filterFiscalPeriod(newReports, opts.fiscalYear, opts.fiscalQuarter)
	fmt.Printf("%s: discovered=%d new=%d existing=%d", check.name, len(reports.Reports), len(newReports), len(reports.Reports)-len(newReports))
	if opts.fiscalYear != 0 {
		fmt.Printf(" filtered=FY%d Q%d", opts.fiscalYear, opts.fiscalQuarter)
	}
	fmt.Println()
	for _, report := range newReports {
		fmt.Printf("  pending FY%d Q%d %s\n", report.FiscalYear, report.FiscalQuarter, report.URL)
	}
	if !opts.apply {
		return nil
	}

	selected := limitReports(newReports, opts.limit)
	fmt.Printf("%s: applying=%d limit=%d\n", check.name, len(selected), opts.limit)
	for _, report := range selected {
		var result load.DetailLoadResult
		switch check.domain {
		case load.ReportDomainMP:
			subjects, profiles, err := mpReportSubjects(service, report)
			if err != nil {
				return fmt.Errorf("build MP FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			profileResult, err := loader.InsertNewMPProfiles(ctx, profiles)
			if err != nil {
				return fmt.Errorf("load MP identities FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			result, err = loader.LoadMPReport(ctx, report, subjects)
			if err != nil {
				return fmt.Errorf("load MP FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			fmt.Printf("  MP identities FY%d Q%d received=%d inserted=%d existing=%d duplicates=%d\n", report.FiscalYear, report.FiscalQuarter, profileResult.Received, profileResult.Inserted, profileResult.Existing, profileResult.DuplicateInput)
		case load.ReportDomainAdministration:
			parsed, err := administrationReport(service, report)
			if err != nil {
				return fmt.Errorf("build Administration FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			result, err = loader.LoadAdministrationReport(ctx, parsed)
			if err != nil {
				return fmt.Errorf("load Administration FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
		case load.ReportDomainOfficer:
			subjects, err := officerReportSubjects(service, report)
			if err != nil {
				return fmt.Errorf("build Officer FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			result, err = loader.LoadOfficerReport(ctx, report, subjects)
			if err != nil {
				return fmt.Errorf("load Officer FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
		default:
			return fmt.Errorf("apply is not implemented for %s reports", check.name)
		}
		fmt.Printf("  loaded FY%d Q%d reports=%d expenses=%d children=%d existing=%d\n", report.FiscalYear, report.FiscalQuarter, result.Reports, result.Expenses, result.ChildRecords, result.ExistingRows)
	}
	return nil
}

// mpReportSubjects builds one complete MP report payload. Historic expenditure
// reports do not expose House member profile URLs, and the current House member
// directory is not a complete historical roster. A source-scoped identity from
// the quarterly report URL, period, row name, and constituency prevents unsafe
// cross-period matching to a current or similarly named former member while
// allowing every disclosed subject to be loaded.
func mpReportSubjects(service *extract.ExtractSvc, report types.ExpenditureReport) ([]load.MPReportSubject, []*types.MPDetails, error) {
	response, err := service.GetByURL(report.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch report page: %w", err)
	}
	doc, parseErr := goquery.NewDocumentFromReader(response.Body)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return nil, nil, fmt.Errorf("parse report page: %w", parseErr)
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("close report page: %w", closeErr)
	}

	summaries, err := transform.Mps(doc, report.FiscalYear, report.DateRange, report.FiscalQuarter)
	if err != nil {
		return nil, nil, fmt.Errorf("transform MP summaries: %w", err)
	}
	subjects := make([]load.MPReportSubject, 0, len(summaries))
	profiles := make([]*types.MPDetails, 0, len(summaries))
	seen := make(map[string]struct{}, len(summaries))
	for _, summary := range summaries {
		if summary == nil {
			return nil, nil, fmt.Errorf("report contains a nil MP summary")
		}
		summary.URL = report.URL
		identity, err := mpSourceIdentity(*summary)
		if err != nil {
			return nil, nil, err
		}
		expenses, err := mpExpenses(service, *summary)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch %s %s details: %w", summary.Name.FirstName, summary.Name.LastName, err)
		}
		subjects = append(subjects, load.MPReportSubject{ProfileURL: identity, Summary: *summary, Expenses: expenses})
		if _, exists := seen[identity]; !exists {
			profiles = append(profiles, &types.MPDetails{Name: summary.Name, ProfileURL: identity})
			seen[identity] = struct{}{}
		}
	}
	return subjects, profiles, nil
}

func mpExpenses(service *extract.ExtractSvc, summary types.MPExpenseSummary) (types.MPExpenses, error) {
	contracts, err := mpCategory(service, summary.ContractExpenses.URL, transform.MpContractExpenses)
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("contracts: %w", err)
	}
	hospitality, err := mpCategory(service, summary.HospitalityExpenses.URL, transform.MpHospitalityExpenses)
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("hospitality: %w", err)
	}
	travel, err := mpCategory(service, summary.TravelExpenses.URL, transform.MpTravelExpenses)
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("travel: %w", err)
	}
	return types.MPExpenses{
		Totals:                 types.ExpenseTotals{Salaries: summary.Salaries, Contract: summary.ContractExpenses.TotalAmount, Hospitality: summary.HospitalityExpenses.TotalAmount, Travel: summary.TravelExpenses.TotalAmount},
		ContractExpenses:       contracts,
		ContractExpensesURL:    strings.TrimSpace(summary.ContractExpenses.URL),
		HospitalityExpenses:    hospitality,
		HospitalityExpensesURL: strings.TrimSpace(summary.HospitalityExpenses.URL),
		TravelExpenses:         travel,
		TravelExpensesURL:      strings.TrimSpace(summary.TravelExpenses.URL),
	}, nil
}

func mpCategory[T any](service *extract.ExtractSvc, sourceURL string, parse func(*goquery.Document) ([]T, error)) ([]T, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return []T{}, nil
	}
	response, err := service.GetByURL(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	doc, parseErr := goquery.NewDocumentFromReader(response.Body)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parse page: %w", parseErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close page: %w", closeErr)
	}
	entries, err := parse(doc)
	if err != nil {
		return nil, fmt.Errorf("transform page: %w", err)
	}
	if entries == nil {
		return []T{}, nil
	}
	return entries, nil
}

func mpSourceIdentity(summary types.MPExpenseSummary) (string, error) {
	first := strings.ToLower(strings.TrimSpace(summary.Name.FirstName))
	last := strings.ToLower(strings.TrimSpace(summary.Name.LastName))
	riding := strings.ToLower(strings.TrimSpace(summary.Constituency))
	if first == "" && last == "" {
		return "", fmt.Errorf("MP source identity requires a name")
	}
	if riding == "" {
		return "", fmt.Errorf("MP source identity requires a constituency for %s %s", summary.Name.FirstName, summary.Name.LastName)
	}
	reportURL := strings.TrimSpace(summary.URL)
	if reportURL == "" || summary.FiscalYear == 0 || summary.FiscalQuarter == 0 {
		return "", fmt.Errorf("MP source identity requires a report URL and fiscal period")
	}
	payload := fmt.Sprintf("%s\x1f%d\x1f%d\x1f%s\x1f%s\x1f%s", reportURL, summary.FiscalYear, summary.FiscalQuarter, first, last, riding)
	digest := sha256.Sum256([]byte(payload))
	return "source://ourcommons.ca/proactivedisclosure/mp/" + hex.EncodeToString(digest[:]), nil
}

func officerReportSubjects(service *extract.ExtractSvc, report types.ExpenditureReport) ([]load.OfficerReportSubject, error) {
	response, err := service.AdminReportsByURL(report.URL)
	if err != nil {
		return nil, fmt.Errorf("fetch report page: %w", err)
	}
	doc, parseErr := goquery.NewDocumentFromReader(response.Body)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parse report page: %w", parseErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close report page: %w", closeErr)
	}

	summaries, err := transform.Officers(doc, report.FiscalYear, report.DateRange, report.FiscalQuarter)
	if err != nil {
		return nil, fmt.Errorf("transform report summaries: %w", err)
	}
	subjects := make([]load.OfficerReportSubject, 0, len(summaries))
	for _, summary := range summaries {
		if summary == nil {
			return nil, fmt.Errorf("report contains a nil officer summary")
		}
		expenses, err := officerExpenses(service, *summary)
		if err != nil {
			return nil, fmt.Errorf("fetch %s %s details: %w", summary.Name.FirstName, summary.Name.LastName, err)
		}
		subjects = append(subjects, load.OfficerReportSubject{Summary: *summary, Expenses: expenses})
	}
	if len(subjects) == 0 {
		return nil, fmt.Errorf("no officer subjects found")
	}
	return subjects, nil
}

func officerExpenses(service *extract.ExtractSvc, summary types.OfficerExpenseSummary) (types.OfficerExpenses, error) {
	contractURL := strings.TrimSpace(summary.ContractExpenses.URL)
	hospitalityURL := strings.TrimSpace(summary.HospitalityExpenses.URL)
	travelURL := strings.TrimSpace(summary.TravelExpenses.URL)

	var contracts []types.OfficerContractExpense
	if contractURL != "" {
		entries, err := officerCategory(service, contractURL, transform.OfficerContractExpenses)
		if err != nil {
			return types.OfficerExpenses{}, fmt.Errorf("contracts: %w", err)
		}
		contracts = entries
	}
	var hospitality []types.OfficerHospitalityExpense
	if hospitalityURL != "" {
		entries, err := officerCategory(service, hospitalityURL, transform.OfficerHospitalityExpenses)
		if err != nil {
			return types.OfficerExpenses{}, fmt.Errorf("hospitality: %w", err)
		}
		hospitality = entries
	}
	var travel []types.OfficerTravelExpense
	if travelURL != "" {
		entries, err := officerCategory(service, travelURL, transform.OfficerTravelExpenses)
		if err != nil {
			return types.OfficerExpenses{}, fmt.Errorf("travel: %w", err)
		}
		travel = entries
	}
	return types.OfficerExpenses{
		ContractExpenses:       contracts,
		ContractExpensesURL:    contractURL,
		HospitalityExpenses:    hospitality,
		HospitalityExpensesURL: hospitalityURL,
		TravelExpenses:         travel,
		TravelExpensesURL:      travelURL,
	}, nil
}

func officerCategory[T any](service *extract.ExtractSvc, sourceURL string, parse func(*goquery.Document) ([]T, error)) ([]T, error) {
	response, err := service.GetByURL(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch page: %w", err)
	}
	doc, parseErr := goquery.NewDocumentFromReader(response.Body)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return nil, fmt.Errorf("parse page: %w", parseErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close page: %w", closeErr)
	}
	entries, err := parse(doc)
	if err != nil {
		return nil, fmt.Errorf("transform page: %w", err)
	}
	return entries, nil
}

func filterFiscalPeriod(reports []types.ExpenditureReport, fiscalYear, fiscalQuarter int) []types.ExpenditureReport {
	if fiscalYear == 0 {
		return reports
	}
	filtered := make([]types.ExpenditureReport, 0, 1)
	for _, report := range reports {
		if report.FiscalYear == fiscalYear && report.FiscalQuarter == fiscalQuarter {
			filtered = append(filtered, report)
		}
	}
	return filtered
}

func limitReports(reports []types.ExpenditureReport, limit int) []types.ExpenditureReport {
	if limit <= 0 {
		return []types.ExpenditureReport{}
	}
	if len(reports) <= limit {
		return reports
	}
	return reports[:limit]
}

func administrationReport(service *extract.ExtractSvc, report types.ExpenditureReport) (types.AdminReport, error) {
	response, err := service.AdminReportsByURL(report.URL)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("fetch report page: %w", err)
	}
	urls, parseErr := transform.AdminExpenseUrls(response)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return types.AdminReport{}, fmt.Errorf("parse report category URLs: %w", parseErr)
	}
	if closeErr != nil {
		return types.AdminReport{}, fmt.Errorf("close report page: %w", closeErr)
	}

	responses, err := service.AllMpExpenses(types.ExpenseURLs{Contract: urls.Contract, Hospitality: urls.Hospitality, Travel: urls.Travel})
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("fetch category pages: %w", err)
	}
	defer closeResponseBodies(responses)

	contractDoc, err := goquery.NewDocumentFromReader(responses.Contract.Body)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("parse contract page: %w", err)
	}
	hospitalityDoc, err := goquery.NewDocumentFromReader(responses.Hospitality.Body)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("parse hospitality page: %w", err)
	}
	travelDoc, err := goquery.NewDocumentFromReader(responses.Travel.Body)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("parse travel page: %w", err)
	}

	contracts, err := transform.AdminContractExpenses(contractDoc)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("transform contracts: %w", err)
	}
	hospitality, err := transform.AdminHospitalityExpenses(hospitalityDoc)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("transform hospitality: %w", err)
	}
	travel, err := transform.AdminTravelExpenses(travelDoc)
	if err != nil {
		return types.AdminReport{}, fmt.Errorf("transform travel: %w", err)
	}

	return types.AdminReport{
		Report:      report,
		Contract:    types.AdminContractExpenseCollection{URL: urls.Contract, Entries: contracts},
		Hospitality: types.AdminHospitalityExpenseCollection{URL: urls.Hospitality, Entries: hospitality},
		Travel:      types.AdminTravelExpenseCollection{URL: urls.Travel, Entries: travel},
	}, nil
}

func closeResponseBodies(responses *types.RawExpenseResponses) {
	if responses == nil {
		return
	}
	for _, response := range []*http.Response{responses.Contract, responses.Hospitality, responses.Travel} {
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
	}
}

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
}
