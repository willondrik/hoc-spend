package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/willondrik/hoc-spend/pkg/scraper/load"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

type auditResult struct {
	Checked int
	Failed  int
	Details []string
}

func (r *auditResult) check(condition bool, detail string) {
	r.Checked++
	if !condition {
		r.Failed++
		r.Details = append(r.Details, detail)
	}
}

func (r *auditResult) merge(other auditResult) {
	r.Checked += other.Checked
	r.Failed += other.Failed
	r.Details = append(r.Details, other.Details...)
}

func (r auditResult) String() string {
	if r.Failed == 0 {
		return fmt.Sprintf("checks=%d status=PASS", r.Checked)
	}
	return fmt.Sprintf("checks=%d failed=%d status=FAIL", r.Checked, r.Failed)
}

type domainRunStats struct {
	Processed    int
	Reports      int
	Subjects     int
	Expenses     int
	ChildRecords int
	ExistingRows int
	Audit        auditResult
}

func (s *domainRunStats) add(result load.DetailLoadResult, audit auditResult) {
	s.Processed++
	s.Reports += result.Reports
	s.Subjects += result.Subjects
	s.Expenses += result.Expenses
	s.ChildRecords += result.ChildRecords
	s.ExistingRows += result.ExistingRows
	s.Audit.merge(audit)
}

func auditMPParsed(subjects []load.MPReportSubject) auditResult {
	result := auditResult{}
	for _, subject := range subjects {
		result.checkTotal("parsed MP", mpSubjectLabel(subject), "contract", subject.Summary.ContractExpenses.TotalAmount, sumMPContracts(subject.Expenses.ContractExpenses))
		result.checkTotal("parsed MP", mpSubjectLabel(subject), "hospitality", subject.Summary.HospitalityExpenses.TotalAmount, sumMPHospitality(subject.Expenses.HospitalityExpenses))
		result.checkTotal("parsed MP", mpSubjectLabel(subject), "travel", subject.Summary.TravelExpenses.TotalAmount, sumMPTravel(subject.Expenses.TravelExpenses))
	}
	return result
}

func auditOfficerParsed(subjects []load.OfficerReportSubject) auditResult {
	result := auditResult{}
	for _, subject := range subjects {
		result.checkTotal("parsed Officer", officerSubjectLabel(subject), "contract", subject.Summary.ContractExpenses.TotalAmount, sumOfficerContracts(subject.Expenses.ContractExpenses))
		result.checkTotal("parsed Officer", officerSubjectLabel(subject), "hospitality", subject.Summary.HospitalityExpenses.TotalAmount, sumOfficerHospitality(subject.Expenses.HospitalityExpenses))
		result.checkTotal("parsed Officer", officerSubjectLabel(subject), "travel", subject.Summary.TravelExpenses.TotalAmount, sumOfficerTravel(subject.Expenses.TravelExpenses))
	}
	return result
}

// Administration currently has no parsed quarterly summary totals. Its audit
// therefore verifies that every parsed category row was persisted below.
func auditAdministrationParsed(report types.AdminReport) auditResult {
	return auditResult{}
}

func (r *auditResult) checkTotal(scope, subject, category string, expected, actual decimal.Decimal) {
	r.check(expected.Equal(actual), fmt.Sprintf("%s %s category=%s summary=%s items=%s delta=%s", scope, subject, category, expected.StringFixed(2), actual.StringFixed(2), actual.Sub(expected).StringFixed(2)))
}

func mpSubjectLabel(subject load.MPReportSubject) string {
	return fmt.Sprintf("name=%q constituency=%q", strings.TrimSpace(subject.Summary.Name.FirstName+" "+subject.Summary.Name.LastName), subject.Summary.Constituency)
}

func officerSubjectLabel(subject load.OfficerReportSubject) string {
	return fmt.Sprintf("name=%q role=%q", strings.TrimSpace(subject.Summary.Name.FirstName+" "+subject.Summary.Name.LastName), subject.Summary.Role)
}

func sumMPContracts(expenses []*types.ContractExpense) decimal.Decimal {
	total := decimal.Zero
	for _, expense := range expenses {
		if expense != nil {
			total = total.Add(expense.Total)
		}
	}
	return total
}

func sumMPHospitality(expenses []*types.HospitalityExpense) decimal.Decimal {
	total := decimal.Zero
	for _, expense := range expenses {
		if expense != nil {
			total = total.Add(expense.TotalCost)
		}
	}
	return total
}

func sumMPTravel(expenses []*types.TravelExpense) decimal.Decimal {
	total := decimal.Zero
	for _, expense := range expenses {
		if expense != nil {
			total = total.Add(expense.Costs.Total)
		}
	}
	return total
}

func sumOfficerContracts(expenses []types.OfficerContractExpense) decimal.Decimal {
	total := decimal.Zero
	for _, expense := range expenses {
		total = total.Add(expense.TotalCost)
	}
	return total
}

func sumOfficerHospitality(expenses []types.OfficerHospitalityExpense) decimal.Decimal {
	total := decimal.Zero
	for _, expense := range expenses {
		total = total.Add(expense.TotalCost)
	}
	return total
}

func sumOfficerTravel(expenses []types.OfficerTravelExpense) decimal.Decimal {
	total := decimal.Zero
	for _, expense := range expenses {
		total = total.Add(expense.TotalCost)
	}
	return total
}

func auditMPPersisted(ctx context.Context, pool *pgxpool.Pool, report types.ExpenditureReport, subjects []load.MPReportSubject) auditResult {
	profileURLs := make([]string, 0, len(subjects))
	for _, subject := range subjects {
		profileURLs = append(profileURLs, subject.ProfileURL)
	}
	return auditPersistedSubjects(ctx, pool, load.ReportDomainMP, report, len(subjects), profileURLs)
}

func auditOfficerPersisted(ctx context.Context, pool *pgxpool.Pool, report types.ExpenditureReport, subjects []load.OfficerReportSubject) auditResult {
	return auditPersistedSubjects(ctx, pool, load.ReportDomainOfficer, report, len(subjects), nil)
}

func auditPersistedSubjects(ctx context.Context, pool *pgxpool.Pool, domain load.ReportDomain, report types.ExpenditureReport, expectedSubjects int, profileURLs []string) auditResult {
	result := auditResult{}
	var subjects int
	err := pool.QueryRow(ctx, `
SELECT
    COUNT(*)
FROM report_subjects
JOIN disclosure_reports ON disclosure_reports.id = report_subjects.report_id
LEFT JOIN mp_affiliations ON mp_affiliations.id = report_subjects.mp_affiliation_id
LEFT JOIN mps ON mps.id = mp_affiliations.mp_id
WHERE disclosure_reports.domain = $1
  AND disclosure_reports.fiscal_year = $2
  AND disclosure_reports.fiscal_quarter = $3
  AND ($4::text[] IS NULL OR mps.profile_url = ANY($4::text[]))`, domain, report.FiscalYear, report.FiscalQuarter, profileURLs).Scan(&subjects)
	result.check(err == nil && subjects == expectedSubjects, fmt.Sprintf("persisted %s subject-count expected=%d actual=%d error=%v", domain, expectedSubjects, subjects, err))
	if err != nil {
		return result
	}

	rows, err := pool.Query(ctx, `
SELECT
    report_subjects.id,
    report_subjects.contract_total::text, COALESCE(contracts.total, 0)::text,
    report_subjects.hospitality_total::text, COALESCE(hospitality.total, 0)::text,
    report_subjects.travel_total::text, COALESCE(travel.total, 0)::text
FROM report_subjects
JOIN disclosure_reports ON disclosure_reports.id = report_subjects.report_id
LEFT JOIN mp_affiliations ON mp_affiliations.id = report_subjects.mp_affiliation_id
LEFT JOIN mps ON mps.id = mp_affiliations.mp_id
LEFT JOIN (
    SELECT report_subject_id, SUM(`+subjectContractTotalColumn(domain)+`) AS total
    FROM `+subjectContractTable(domain)+`
    GROUP BY report_subject_id
) contracts ON contracts.report_subject_id = report_subjects.id
LEFT JOIN (
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM `+subjectHospitalityTable(domain)+`
    GROUP BY report_subject_id
) hospitality ON hospitality.report_subject_id = report_subjects.id
LEFT JOIN (
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM `+subjectTravelTable(domain)+`
    GROUP BY report_subject_id
) travel ON travel.report_subject_id = report_subjects.id
WHERE disclosure_reports.domain = $1
  AND disclosure_reports.fiscal_year = $2
  AND disclosure_reports.fiscal_quarter = $3
  AND ($4::text[] IS NULL OR mps.profile_url = ANY($4::text[]))
  AND (
      report_subjects.contract_total <> COALESCE(contracts.total, 0)
      OR report_subjects.hospitality_total <> COALESCE(hospitality.total, 0)
      OR report_subjects.travel_total <> COALESCE(travel.total, 0)
  )`, domain, report.FiscalYear, report.FiscalQuarter, profileURLs)
	if err != nil {
		result.check(false, fmt.Sprintf("persisted %s total query error=%v", domain, err))
		return result
	}
	defer rows.Close()
	persistedMismatch := false
	for rows.Next() {
		var id int64
		var contractExpected, contractActual, hospitalityExpected, hospitalityActual, travelExpected, travelActual string
		if err := rows.Scan(&id, &contractExpected, &contractActual, &hospitalityExpected, &hospitalityActual, &travelExpected, &travelActual); err != nil {
			result.check(false, fmt.Sprintf("persisted %s total scan error=%v", domain, err))
			continue
		}
		persistedMismatch = result.appendPersistedMismatch(domain, id, "contract", contractExpected, contractActual) || persistedMismatch
		persistedMismatch = result.appendPersistedMismatch(domain, id, "hospitality", hospitalityExpected, hospitalityActual) || persistedMismatch
		persistedMismatch = result.appendPersistedMismatch(domain, id, "travel", travelExpected, travelActual) || persistedMismatch
	}
	if err := rows.Err(); err != nil {
		result.check(false, fmt.Sprintf("persisted %s total iteration error=%v", domain, err))
		return result
	}
	result.check(!persistedMismatch, fmt.Sprintf("persisted %s category totals disagree with detail rows", domain))
	return result
}

func (r *auditResult) appendPersistedMismatch(domain load.ReportDomain, subjectID int64, category, expectedText, actualText string) bool {
	expected, expectedErr := decimal.NewFromString(expectedText)
	actual, actualErr := decimal.NewFromString(actualText)
	if expectedErr != nil || actualErr != nil {
		r.Details = append(r.Details, fmt.Sprintf("persisted %s subject_id=%d category=%s decimal parse errors expected=%q actual=%q", domain, subjectID, category, expectedText, actualText))
		return true
	}
	if expected.Equal(actual) {
		return false
	}
	r.Details = append(r.Details, fmt.Sprintf("persisted %s subject_id=%d category=%s summary=%s items=%s delta=%s", domain, subjectID, category, expected.StringFixed(2), actual.StringFixed(2), actual.Sub(expected).StringFixed(2)))
	return true
}

func auditAdministrationPersisted(ctx context.Context, pool *pgxpool.Pool, report types.ExpenditureReport, parsed types.AdminReport) auditResult {
	result := auditResult{}
	var contracts, hospitality, travel int
	err := pool.QueryRow(ctx, `
SELECT
    (SELECT COUNT(*) FROM administration_contract_expenses WHERE report_id = disclosure_reports.id),
    (SELECT COUNT(*) FROM administration_hospitality_expenses WHERE report_id = disclosure_reports.id),
    (SELECT COUNT(*) FROM administration_travel_expenses WHERE report_id = disclosure_reports.id)
FROM disclosure_reports
WHERE domain = $1 AND fiscal_year = $2 AND fiscal_quarter = $3`, load.ReportDomainAdministration, report.FiscalYear, report.FiscalQuarter).Scan(&contracts, &hospitality, &travel)
	result.check(err == nil && contracts == len(parsed.Contract.Entries), fmt.Sprintf("persisted administration contract row-count parsed=%d stored=%d error=%v", len(parsed.Contract.Entries), contracts, err))
	result.check(err == nil && hospitality == len(parsed.Hospitality.Entries), fmt.Sprintf("persisted administration hospitality row-count parsed=%d stored=%d error=%v", len(parsed.Hospitality.Entries), hospitality, err))
	result.check(err == nil && travel == len(parsed.Travel.Entries), fmt.Sprintf("persisted administration travel row-count parsed=%d stored=%d error=%v", len(parsed.Travel.Entries), travel, err))
	return result
}

func subjectContractTable(domain load.ReportDomain) string {
	if domain == load.ReportDomainMP {
		return "mp_contract_expenses"
	}
	return "officer_contract_expenses"
}

func subjectHospitalityTable(domain load.ReportDomain) string {
	if domain == load.ReportDomainMP {
		return "mp_hospitality_expenses"
	}
	return "officer_hospitality_expenses"
}

func subjectContractTotalColumn(domain load.ReportDomain) string {
	if domain == load.ReportDomainMP {
		return "total"
	}
	return "total_cost"
}

func subjectTravelTable(domain load.ReportDomain) string {
	if domain == load.ReportDomainMP {
		return "mp_travel_expenses"
	}
	return "officer_travel_expenses"
}
