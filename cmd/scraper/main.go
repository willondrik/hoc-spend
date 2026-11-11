// Command scraper discovers House of Commons expenditure reports and compares
// them with the configured PostgreSQL database.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jackc/pgx/v5/pgxpool"
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
	domain           string
	apply            bool
	audit            bool
	enrichMPProfiles bool
	allReports       bool
	limit            int
	fiscalYear       int
	fiscalQuarter    int
}

func main() {
	var (
		domain               = flag.String("domain", "all", "report domain to inspect: all, mp, administration, or officer")
		apply                = flag.Bool("apply", false, "fetch, parse, and load new reports")
		audit                = flag.Bool("audit", false, "verify parsed and persisted category totals; requires -apply")
		enrichMPProfilesFlag = flag.Bool("enrich-mp-profiles", false, "fetch current House MP profiles and enrich uniquely matching loaded disclosure MPs")
		allReports           = flag.Bool("all", false, "with -apply, process discovered reports including existing reports; ignores -limit")
		limit                = flag.Int("limit", 1, "maximum new reports to load with -apply; zero loads none")
		fiscalYear           = flag.Int("fiscal-year", 0, "optional fiscal year filter; requires -fiscal-quarter")
		fiscalQuarter        = flag.Int("fiscal-quarter", 0, "optional fiscal quarter filter (1-4); requires -fiscal-year")
	)
	flag.Parse()

	opts := runOptions{
		domain:           *domain,
		apply:            *apply,
		audit:            *audit,
		enrichMPProfiles: *enrichMPProfilesFlag,
		allReports:       *allReports,
		limit:            *limit,
		fiscalYear:       *fiscalYear,
		fiscalQuarter:    *fiscalQuarter,
	}
	if err := opts.validate(); err != nil {
		log.Fatal(err)
	}

	timeout := 2 * time.Minute
	if opts.apply || opts.enrichMPProfiles {
		timeout = 2 * time.Hour
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
	if opts.enrichMPProfiles {
		if err := enrichMPProfiles(ctx, loader, service); err != nil {
			log.Fatal(err)
		}
		return
	}

	checks := []discovery{
		{name: "MP", domain: load.ReportDomainMP, fetch: service.MpReports, parse: transform.MpExpenditureReports},
		{name: "Administration", domain: load.ReportDomainAdministration, fetch: service.AdminReports, parse: transform.AdministratorExpenditureReports},
		{name: "Officer", domain: load.ReportDomainOfficer, fetch: service.OfficerReports, parse: transform.OfficerExpenditureReports},
	}

	selected, err := selectDiscoveries(*domain, checks)
	if err != nil {
		log.Fatal(err)
	}
	var runErrors []error
	for _, check := range selected {
		if err := runDomain(ctx, database.Pool, loader, service, check, opts); err != nil {
			log.Printf("%v", err)
			runErrors = append(runErrors, err)
		}
	}
	if len(runErrors) > 0 {
		log.Fatal(errors.Join(runErrors...))
	}
}

func (opts runOptions) validate() error {
	if opts.limit < 0 {
		return fmt.Errorf("-limit must be zero or greater")
	}
	if opts.audit && !opts.apply {
		return fmt.Errorf("-audit requires -apply so persisted data can be verified")
	}
	if opts.enrichMPProfiles && (opts.apply || opts.audit || opts.allReports || opts.fiscalYear != 0 || opts.fiscalQuarter != 0 || opts.domain != "all" || opts.limit != 1) {
		return fmt.Errorf("-enrich-mp-profiles cannot be combined with report-loading, audit, domain, limit, or fiscal-period flags")
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

func runDomain(ctx context.Context, pool *pgxpool.Pool, loader *load.Loader, service *extract.ExtractSvc, check discovery, opts runOptions) error {
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

	allReports := filterFiscalPeriod(reports.Reports, opts.fiscalYear, opts.fiscalQuarter)
	newReports = filterFiscalPeriod(newReports, opts.fiscalYear, opts.fiscalQuarter)
	fmt.Printf("%s: discovered=%d new=%d existing=%d", check.name, len(allReports), len(newReports), len(allReports)-len(newReports))
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
	if opts.allReports {
		selected = allReports
	}
	fmt.Printf("%s: applying=%d", check.name, len(selected))
	if opts.allReports {
		fmt.Print(" scope=all-discovered")
	} else {
		fmt.Printf(" limit=%d", opts.limit)
	}
	if opts.audit {
		fmt.Print(" audit=enabled")
	}
	fmt.Println()
	stats := domainRunStats{}
	for _, report := range selected {
		var result load.DetailLoadResult
		var audit auditResult
		switch check.domain {
		case load.ReportDomainMP:
			subjects, profiles, err := mpReportSubjects(service, report)
			if err != nil {
				return fmt.Errorf("build MP FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			if err := loader.ReuseExistingMPSubjectProfiles(ctx, report, subjects); err != nil {
				return fmt.Errorf("reuse MP identities FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			profiles = mpProfilesFromSubjects(subjects)
			profileResult, err := loader.InsertNewMPProfiles(ctx, profiles)
			if err != nil {
				return fmt.Errorf("load MP identities FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			result, err = loader.LoadMPReport(ctx, report, subjects)
			if err != nil {
				return fmt.Errorf("load MP FY%d Q%d: %w", report.FiscalYear, report.FiscalQuarter, err)
			}
			if opts.audit {
				audit = auditMPParsed(report, subjects)
				audit.merge(auditMPPersisted(ctx, pool, report, subjects))
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
			if opts.audit {
				audit = auditAdministrationParsed(parsed)
				audit.merge(auditAdministrationPersisted(ctx, pool, report, parsed))
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
			if opts.audit {
				audit = auditOfficerParsed(report, subjects)
				audit.merge(auditOfficerPersisted(ctx, pool, report, subjects))
			}
		default:
			return fmt.Errorf("apply is not implemented for %s reports", check.name)
		}
		stats.add(result, audit)
		fmt.Printf("  loaded FY%d Q%d reports=%d subjects=%d expenses=%d children=%d existing=%d\n", report.FiscalYear, report.FiscalQuarter, result.Reports, result.Subjects, result.Expenses, result.ChildRecords, result.ExistingRows)
		if opts.audit {
			fmt.Printf("  audit FY%d Q%d %s\n", report.FiscalYear, report.FiscalQuarter, audit)
			for _, detail := range audit.Details {
				fmt.Printf("    audit failure: %s\n", detail)
			}
			for _, detail := range audit.WarningDetails {
				fmt.Printf("    audit warning: %s\n", detail)
			}
		}
	}
	fmt.Printf("%s: processed=%d reports=%d subjects=%d expenses=%d children=%d existing=%d", check.name, stats.Processed, stats.Reports, stats.Subjects, stats.Expenses, stats.ChildRecords, stats.ExistingRows)
	if opts.audit {
		fmt.Printf(" audit-checks=%d audit-failed=%d audit-warnings=%d", stats.Audit.Checked, stats.Audit.Failed, stats.Audit.Warnings)
	}
	fmt.Println()
	if opts.audit && stats.Audit.Failed > 0 {
		return fmt.Errorf("audit failed for %s: %d of %d checks failed", check.name, stats.Audit.Failed, stats.Audit.Checked)
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
	for index, summary := range summaries {
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
		if (index+1)%25 == 0 || index+1 == len(summaries) {
			fmt.Printf("  fetched MP details %d/%d\n", index+1, len(summaries))
		}
		subjects = append(subjects, load.MPReportSubject{SourceIdentity: identity, Summary: *summary, Expenses: expenses})
		if _, exists := seen[identity]; !exists {
			profiles = append(profiles, &types.MPDetails{Name: summary.Name, SourceIdentity: identity})
			seen[identity] = struct{}{}
		}
	}
	return subjects, profiles, nil
}

func mpExpenses(service *extract.ExtractSvc, summary types.MPExpenseSummary) (types.MPExpenses, error) {
	responses, err := service.AllMpExpenses(types.ExpenseURLs{
		Contract:    strings.TrimSpace(summary.ContractExpenses.URL),
		Hospitality: strings.TrimSpace(summary.HospitalityExpenses.URL),
		Travel:      strings.TrimSpace(summary.TravelExpenses.URL),
	})
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("fetch category pages: %w", err)
	}
	defer closeResponseBodies(responses)

	contracts, contractCSVURL, err := mpResponseCategory(responses.Contract, transform.MpContractExpenses)
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("contracts: %w", err)
	}
	hospitality, hospitalityCSVURL, err := mpResponseCategory(responses.Hospitality, transform.MpHospitalityExpenses)
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("hospitality: %w", err)
	}
	travel, travelCSVURL, err := mpResponseCategory(responses.Travel, transform.MpTravelExpenses)
	if err != nil {
		return types.MPExpenses{}, fmt.Errorf("travel: %w", err)
	}
	return types.MPExpenses{
		Totals:                    types.ExpenseTotals{Salaries: summary.Salaries, Contract: summary.ContractExpenses.TotalAmount, Hospitality: summary.HospitalityExpenses.TotalAmount, Travel: summary.TravelExpenses.TotalAmount},
		ContractExpenses:          contracts,
		ContractExpensesURL:       strings.TrimSpace(summary.ContractExpenses.URL),
		ContractExpensesCSVURL:    contractCSVURL,
		HospitalityExpenses:       hospitality,
		HospitalityExpensesURL:    strings.TrimSpace(summary.HospitalityExpenses.URL),
		HospitalityExpensesCSVURL: hospitalityCSVURL,
		TravelExpenses:            travel,
		TravelExpensesURL:         strings.TrimSpace(summary.TravelExpenses.URL),
		TravelExpensesCSVURL:      travelCSVURL,
	}, nil
}

func mpResponseCategory[T any](response *http.Response, parse func(*goquery.Document) ([]T, error)) ([]T, string, error) {
	if response == nil {
		return []T{}, "", nil
	}
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, "", fmt.Errorf("parse page: %w", err)
	}
	entries, err := parse(doc)
	if err != nil {
		return nil, "", fmt.Errorf("transform page: %w", err)
	}
	if entries == nil {
		return []T{}, transform.CSVDownloadURL(doc), nil
	}
	return entries, transform.CSVDownloadURL(doc), nil
}

func mpProfilesFromSubjects(subjects []load.MPReportSubject) []*types.MPDetails {
	profiles := make([]*types.MPDetails, 0, len(subjects))
	seen := make(map[string]struct{}, len(subjects))
	for _, subject := range subjects {
		sourceIdentity := strings.TrimSpace(subject.SourceIdentity)
		if sourceIdentity == "" {
			continue
		}
		if _, exists := seen[sourceIdentity]; exists {
			continue
		}
		profiles = append(profiles, &types.MPDetails{Name: subject.Summary.Name, SourceIdentity: sourceIdentity})
		seen[sourceIdentity] = struct{}{}
	}
	return profiles
}

func enrichMPProfiles(ctx context.Context, loader *load.Loader, service *extract.ExtractSvc) error {
	response, err := service.MpProfileUrl()
	if err != nil {
		return fmt.Errorf("fetch House MP directory: %w", err)
	}
	doc, parseErr := goquery.NewDocumentFromReader(response.Body)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return fmt.Errorf("parse House MP directory: %w", parseErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close House MP directory: %w", closeErr)
	}
	entries, err := transform.MPDirectoryEntries(doc)
	if err != nil {
		return fmt.Errorf("extract House MP directory entries: %w", err)
	}

	updated, unmatched, ambiguous, failed := 0, 0, 0, 0
	for index, entry := range entries {
		response, err := service.MpProfileByURL(entry.ProfileURL)
		if err != nil {
			failed++
			fmt.Printf("  MP profile fetch failed name=%q url=%s error=%v\n", strings.TrimSpace(entry.Name.FirstName+" "+entry.Name.LastName), entry.ProfileURL, err)
			continue
		}
		profile, parseErr := transform.GetProfile(response, entry.ProfileURL)
		closeErr := response.Body.Close()
		if parseErr != nil || closeErr != nil {
			failed++
			fmt.Printf("  MP profile parse failed name=%q url=%s error=%v\n", strings.TrimSpace(entry.Name.FirstName+" "+entry.Name.LastName), entry.ProfileURL, errors.Join(parseErr, closeErr))
			continue
		}
		// Use the directory tile name for matching. It is paired with the
		// directory constituency and is less susceptible to profile-page markup
		// variations such as honorifics.
		profile.Name = entry.Name
		result, err := loader.EnrichMPProfile(ctx, profile, entry.Constituency)
		if err != nil {
			failed++
			fmt.Printf("  MP profile load failed name=%q constituency=%q error=%v\n", strings.TrimSpace(entry.Name.FirstName+" "+entry.Name.LastName), entry.Constituency, err)
			continue
		}
		switch {
		case result.Updated:
			updated++
		case result.Unmatched:
			unmatched++
		case result.Ambiguous:
			ambiguous++
		}
		if (index+1)%25 == 0 || index+1 == len(entries) {
			fmt.Printf("  enriched current MP profiles %d/%d updated=%d unmatched=%d ambiguous=%d failed=%d\n", index+1, len(entries), updated, unmatched, ambiguous, failed)
		}
	}
	fmt.Printf("MP profile enrichment: discovered=%d updated=%d unmatched=%d ambiguous=%d failed=%d\n", len(entries), updated, unmatched, ambiguous, failed)
	if failed > 0 {
		return fmt.Errorf("MP profile enrichment completed with %d failed profile requests/parses", failed)
	}
	return nil
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
	// The House summaryId embedded in a quarterly report URL changes when the
	// source republishes the report. Including that URL or fiscal period here
	// would create a second durable MP identity for the same named MP/riding on
	// each re-scrape. The fallback therefore remains stable across periods while
	// retaining constituency to avoid merging same-name MPs from different ridings.
	payload := fmt.Sprintf("%s\x1f%s\x1f%s", first, last, riding)
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
	var contractCSVURL string
	if contractURL != "" {
		entries, csvURL, err := officerCategory(service, contractURL, transform.OfficerContractExpenses)
		if err != nil {
			return types.OfficerExpenses{}, fmt.Errorf("contracts: %w", err)
		}
		contracts = entries
		contractCSVURL = csvURL
	}
	var hospitality []types.OfficerHospitalityExpense
	var hospitalityCSVURL string
	if hospitalityURL != "" {
		entries, csvURL, err := officerCategory(service, hospitalityURL, transform.OfficerHospitalityExpenses)
		if err != nil {
			return types.OfficerExpenses{}, fmt.Errorf("hospitality: %w", err)
		}
		hospitality = entries
		hospitalityCSVURL = csvURL
	}
	var travel []types.OfficerTravelExpense
	var travelCSVURL string
	if travelURL != "" {
		entries, csvURL, err := officerCategory(service, travelURL, transform.OfficerTravelExpenses)
		if err != nil {
			return types.OfficerExpenses{}, fmt.Errorf("travel: %w", err)
		}
		travel = entries
		travelCSVURL = csvURL
	}
	return types.OfficerExpenses{
		ContractExpenses:          contracts,
		ContractExpensesURL:       contractURL,
		ContractExpensesCSVURL:    contractCSVURL,
		HospitalityExpenses:       hospitality,
		HospitalityExpensesURL:    hospitalityURL,
		HospitalityExpensesCSVURL: hospitalityCSVURL,
		TravelExpenses:            travel,
		TravelExpensesURL:         travelURL,
		TravelExpensesCSVURL:      travelCSVURL,
	}, nil
}

func officerCategory[T any](service *extract.ExtractSvc, sourceURL string, parse func(*goquery.Document) ([]T, error)) ([]T, string, error) {
	response, err := service.GetByURL(sourceURL)
	if err != nil {
		return nil, "", fmt.Errorf("fetch page: %w", err)
	}
	doc, parseErr := goquery.NewDocumentFromReader(response.Body)
	closeErr := response.Body.Close()
	if parseErr != nil {
		return nil, "", fmt.Errorf("parse page: %w", parseErr)
	}
	if closeErr != nil {
		return nil, "", fmt.Errorf("close page: %w", closeErr)
	}
	entries, err := parse(doc)
	if err != nil {
		return nil, "", fmt.Errorf("transform page: %w", err)
	}
	return entries, transform.CSVDownloadURL(doc), nil
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
		Contract:    types.AdminContractExpenseCollection{URL: urls.Contract, CSVURL: transform.CSVDownloadURL(contractDoc), Entries: contracts},
		Hospitality: types.AdminHospitalityExpenseCollection{URL: urls.Hospitality, CSVURL: transform.CSVDownloadURL(hospitalityDoc), Entries: hospitality},
		Travel:      types.AdminTravelExpenseCollection{URL: urls.Travel, CSVURL: transform.CSVDownloadURL(travelDoc), Entries: travel},
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
