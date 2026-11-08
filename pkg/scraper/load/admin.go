package load

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

// DetailLoadResult summarizes one idempotent report-detail load.
type DetailLoadResult struct {
	Reports      int
	Subjects     int
	Expenses     int
	ChildRecords int
	ExistingRows int
}

func detailResult(counts loadCounts) DetailLoadResult {
	return DetailLoadResult{Reports: counts.Reports, Subjects: counts.Subjects, Expenses: counts.Expenses, ChildRecords: counts.ChildRecords, ExistingRows: counts.ExistingRows}
}

// LoadAdministrationReport writes one administration report and all of its
// parsed expenses atomically. It only inserts missing source rows.
func (l *Loader) LoadAdministrationReport(ctx context.Context, report types.AdminReport) (DetailLoadResult, error) {
	tx, err := beginLoad(ctx, l)
	if err != nil {
		return DetailLoadResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit makes this a no-op.

	reportID, existing, err := ensureReport(ctx, tx, ReportDomainAdministration, report.Report)
	if err != nil {
		return DetailLoadResult{}, fmt.Errorf("load administration report: %w", err)
	}
	counts := loadCounts{Reports: 1}
	if existing {
		counts.ExistingRows++
	}
	for _, expense := range report.Contract.Entries {
		loaded, err := loadAdministrationContract(ctx, tx, reportID, expense)
		if err != nil {
			return DetailLoadResult{}, err
		}
		counts.add(loaded)
	}
	for _, expense := range report.Hospitality.Entries {
		loaded, err := loadAdministrationHospitality(ctx, tx, reportID, expense)
		if err != nil {
			return DetailLoadResult{}, err
		}
		counts.add(loaded)
	}
	for _, expense := range report.Travel.Entries {
		loaded, err := loadAdministrationTravel(ctx, tx, reportID, expense)
		if err != nil {
			return DetailLoadResult{}, err
		}
		counts.add(loaded)
	}
	if err := tx.Commit(ctx); err != nil {
		return DetailLoadResult{}, fmt.Errorf("commit administration report: %w", err)
	}
	return detailResult(counts), nil
}

func loadAdministrationContract(ctx context.Context, tx pgx.Tx, reportID int64, expense types.AdminContractExpense) (loadCounts, error) {
	key, err := sourceKey("administration-contract", expense)
	if err != nil {
		return loadCounts{}, err
	}
	start, end, sourceStart, sourceEnd := sourceDateRange(expense.ContractPeriod)
	id, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO administration_contract_expenses (report_id, source_key, description, period_start, period_end, source_period_start, source_period_end, original_contract_value, amended_contract_value)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (report_id, source_key) DO NOTHING RETURNING id`,
		`SELECT id FROM administration_contract_expenses WHERE report_id = $1 AND source_key = $2`, reportID, key, sourceText(expense.Description), start, end, sourceStart, sourceEnd, expense.OriginalContractValue, expense.AmendedContractValue)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert administration contract: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for _, supplier := range uniqueStrings(expense.Suppliers) {
		tag, err := tx.Exec(ctx, `INSERT INTO administration_contract_suppliers (contract_expense_id, supplier) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, supplier)
		if err != nil {
			return loadCounts{}, fmt.Errorf("insert administration contract supplier: %w", err)
		}
		counts.ChildRecords += int(tag.RowsAffected())
	}
	return counts, nil
}

func loadAdministrationHospitality(ctx context.Context, tx pgx.Tx, reportID int64, expense types.AdminHospitalityExpense) (loadCounts, error) {
	eventID := sourceText(expense.EventID)
	if eventID == "" {
		return loadCounts{}, fmt.Errorf("administration hospitality event ID is required")
	}
	date, rawDate := sourceDate(expense.Date)
	id, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO administration_hospitality_expenses (report_id, event_id, host, purpose, expense_date, source_date, total_cost, cost_per_person, location, attendance)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (report_id, event_id) DO NOTHING RETURNING id`,
		`SELECT id FROM administration_hospitality_expenses WHERE report_id = $1 AND event_id = $2`, reportID, eventID, sourceText(expense.Host), sourceText(expense.Purpose), date, rawDate, expense.TotalCost, expense.CostPerPerson, sourceText(expense.Location), sourceText(expense.Attendance))
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert administration hospitality: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for _, supplier := range uniqueStrings(expense.Suppliers) {
		tag, err := tx.Exec(ctx, `INSERT INTO administration_hospitality_suppliers (hospitality_expense_id, supplier) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, supplier)
		if err != nil {
			return loadCounts{}, fmt.Errorf("insert administration hospitality supplier: %w", err)
		}
		counts.ChildRecords += int(tag.RowsAffected())
	}
	return counts, nil
}

func loadAdministrationTravel(ctx context.Context, tx pgx.Tx, reportID int64, expense types.AdminTravelExpense) (loadCounts, error) {
	eventID := sourceText(expense.EventID)
	if eventID == "" {
		return loadCounts{}, fmt.Errorf("administration travel event ID is required")
	}
	travellerID, err := ensureOptionalPerson(ctx, tx, expense.Traveller)
	if err != nil {
		return loadCounts{}, fmt.Errorf("resolve administration traveller: %w", err)
	}
	start, end, sourceStart, sourceEnd := sourceDateRange(expense.DateRange)
	id, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO administration_travel_expenses (report_id, event_id, traveller_person_id, purpose, period_start, period_end, source_period_start, source_period_end, total_cost, transportation_cost, accommodations_cost, meals_and_incidentals_cost, other_cost)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (report_id, event_id) DO NOTHING RETURNING id`,
		`SELECT id FROM administration_travel_expenses WHERE report_id = $1 AND event_id = $2`, reportID, eventID, travellerID, sourceText(expense.Purpose), start, end, sourceStart, sourceEnd, expense.TotalCost, expense.ExpenseBreakdown.Transportation, expense.ExpenseBreakdown.Accommodations, expense.ExpenseBreakdown.MealsAndIncidentals, expense.ExpenseBreakdown.Other)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert administration travel: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for position, place := range uniqueOrderedStrings(expense.PlacesVisited) {
		tag, err := tx.Exec(ctx, `INSERT INTO administration_travel_places (travel_expense_id, position, place) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, position, place)
		if err != nil {
			return loadCounts{}, fmt.Errorf("insert administration travel place: %w", err)
		}
		counts.ChildRecords += int(tag.RowsAffected())
	}
	return counts, nil
}
