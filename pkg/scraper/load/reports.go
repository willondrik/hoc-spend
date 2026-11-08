package load

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

// ReportDomain identifies one House of Commons disclosure-report source.
type ReportDomain string

const (
	ReportDomainMP             ReportDomain = "mp"
	ReportDomainAdministration ReportDomain = "administration"
	ReportDomainOfficer        ReportDomain = "officer"
)

// ReportLoadResult describes the outcome of an additive disclosure-report load.
type ReportLoadResult struct {
	Received       int
	DuplicateInput int
	Existing       int
	Inserted       int
}

type existingReports struct {
	urls    map[string]struct{}
	periods map[string]struct{}
}

// NewReports returns only reports that are not already stored for domain. It
// performs no writes, so callers can use it immediately after discovery to
// avoid fetching and parsing detail pages for historic reporting periods.
func (l *Loader) NewReports(ctx context.Context, domain ReportDomain, reports []types.ExpenditureReport) ([]types.ExpenditureReport, error) {
	if !domain.valid() {
		return nil, fmt.Errorf("load reports: unsupported domain %q", domain)
	}

	unique, _, err := uniqueReports(reports)
	if err != nil {
		return nil, err
	}
	if len(unique) == 0 {
		return []types.ExpenditureReport{}, nil
	}

	existing, err := l.existingReports(ctx, domain, unique)
	if err != nil {
		return nil, err
	}
	return filterNewReports(unique, existing), nil
}

// InsertNewReports writes only reports that are not already present for domain.
// Existing reports are identified by either source URL or fiscal year/quarter,
// matching both database uniqueness constraints. Existing rows are never
// updated, so scheduled loads are additive and idempotent.
func (l *Loader) InsertNewReports(ctx context.Context, domain ReportDomain, reports []types.ExpenditureReport) (ReportLoadResult, error) {
	result := ReportLoadResult{Received: len(reports)}
	if !domain.valid() {
		return result, fmt.Errorf("load reports: unsupported domain %q", domain)
	}

	unique, duplicates, err := uniqueReports(reports)
	if err != nil {
		return result, err
	}
	result.DuplicateInput = duplicates
	if len(unique) == 0 {
		return result, nil
	}

	existing, err := l.existingReports(ctx, domain, unique)
	if err != nil {
		return result, err
	}

	newReports := filterNewReports(unique, existing)
	result.Existing = len(unique) - len(newReports)
	if len(newReports) == 0 {
		return result, nil
	}

	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("load reports: begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // A committed transaction cannot be rolled back.

	batch := &pgx.Batch{}
	for _, report := range newReports {
		periodStart, err := parseReportDate(report.DateRange.StartDate)
		if err != nil {
			return result, fmt.Errorf("load report %q: parse period start: %w", report.URL, err)
		}
		periodEnd, err := parseReportDate(report.DateRange.EndDate)
		if err != nil {
			return result, fmt.Errorf("load report %q: parse period end: %w", report.URL, err)
		}
		batch.Queue(`
INSERT INTO disclosure_reports (
    domain, fiscal_year, fiscal_quarter, period_start, period_end, source_url
)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`, domain, report.FiscalYear, report.FiscalQuarter, periodStart, periodEnd, report.URL)
	}

	batchResults := tx.SendBatch(ctx, batch)
	for range newReports {
		tag, err := batchResults.Exec()
		if err != nil {
			batchResults.Close()
			return result, fmt.Errorf("load reports: insert report: %w", err)
		}
		result.Inserted += int(tag.RowsAffected())
	}
	if err := batchResults.Close(); err != nil {
		return result, fmt.Errorf("load reports: close insert batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("load reports: commit transaction: %w", err)
	}

	return result, nil
}

func (l *Loader) existingReports(ctx context.Context, domain ReportDomain, reports []types.ExpenditureReport) (existingReports, error) {
	years := make([]int, 0, len(reports))
	quarters := make([]int16, 0, len(reports))
	urls := make([]string, 0, len(reports))
	for _, report := range reports {
		years = append(years, report.FiscalYear)
		quarters = append(quarters, int16(report.FiscalQuarter))
		urls = append(urls, report.URL)
	}

	rows, err := l.pool.Query(ctx, `
SELECT domain, fiscal_year, fiscal_quarter, source_url
FROM disclosure_reports
WHERE source_url = ANY($2::text[])
   OR (
      domain = $1
      AND (fiscal_year, fiscal_quarter) IN (
          SELECT * FROM unnest($3::int[], $4::smallint[])
      )
   )`, domain, urls, years, quarters)
	if err != nil {
		return existingReports{}, fmt.Errorf("load reports: query existing reports: %w", err)
	}
	defer rows.Close()

	existing := existingReports{
		urls:    make(map[string]struct{}),
		periods: make(map[string]struct{}),
	}
	for rows.Next() {
		var existingDomain ReportDomain
		var fiscalYear, fiscalQuarter int
		var sourceURL string
		if err := rows.Scan(&existingDomain, &fiscalYear, &fiscalQuarter, &sourceURL); err != nil {
			return existingReports{}, fmt.Errorf("load reports: scan existing report: %w", err)
		}
		existing.urls[sourceURL] = struct{}{}
		if existingDomain == domain {
			existing.periods[reportPeriodKey(fiscalYear, fiscalQuarter)] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return existingReports{}, fmt.Errorf("load reports: read existing reports: %w", err)
	}

	return existing, nil
}

func uniqueReports(reports []types.ExpenditureReport) ([]types.ExpenditureReport, int, error) {
	unique := make([]types.ExpenditureReport, 0, len(reports))
	seenURLs := make(map[string]struct{}, len(reports))
	seenPeriods := make(map[string]struct{}, len(reports))
	duplicates := 0

	for index, report := range reports {
		report.URL = strings.TrimSpace(report.URL)
		if report.URL == "" {
			return nil, duplicates, fmt.Errorf("load reports: report at index %d has an empty source URL", index)
		}
		if report.FiscalYear < 1867 || report.FiscalYear > 9999 {
			return nil, duplicates, fmt.Errorf("load reports: report %q has invalid fiscal year %d", report.URL, report.FiscalYear)
		}
		if report.FiscalQuarter < 1 || report.FiscalQuarter > 4 {
			return nil, duplicates, fmt.Errorf("load reports: report %q has invalid fiscal quarter %d", report.URL, report.FiscalQuarter)
		}

		periodKey := reportPeriodKey(report.FiscalYear, report.FiscalQuarter)
		if _, found := seenURLs[report.URL]; found {
			duplicates++
			continue
		}
		if _, found := seenPeriods[periodKey]; found {
			duplicates++
			continue
		}
		seenURLs[report.URL] = struct{}{}
		seenPeriods[periodKey] = struct{}{}
		unique = append(unique, report)
	}

	return unique, duplicates, nil
}

func filterNewReports(reports []types.ExpenditureReport, existing existingReports) []types.ExpenditureReport {
	newReports := make([]types.ExpenditureReport, 0, len(reports))
	for _, report := range reports {
		if _, found := existing.urls[report.URL]; found {
			continue
		}
		if _, found := existing.periods[reportPeriodKey(report.FiscalYear, report.FiscalQuarter)]; found {
			continue
		}
		newReports = append(newReports, report)
	}
	return newReports
}

func reportPeriodKey(fiscalYear, fiscalQuarter int) string {
	return fmt.Sprintf("%d:%d", fiscalYear, fiscalQuarter)
}

func parseReportDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "not provided") {
		return nil, nil
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("expected YYYY-MM-DD or empty value, got %q", value)
	}
	return &parsed, nil
}

func (d ReportDomain) valid() bool {
	return d == ReportDomainMP || d == ReportDomainAdministration || d == ReportDomainOfficer
}
