package load

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

// OfficerReportSubject joins a quarterly officer summary to its parsed details.
type OfficerReportSubject struct {
	Summary  types.OfficerExpenseSummary
	Expenses types.OfficerExpenses
}

// LoadOfficerReport writes a complete House Officer report and all supplied
// officer subjects/details in one transaction. It inserts missing rows only.
func (l *Loader) LoadOfficerReport(ctx context.Context, report types.ExpenditureReport, subjects []OfficerReportSubject) (DetailLoadResult, error) {
	tx, err := beginLoad(ctx, l)
	if err != nil {
		return DetailLoadResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit makes this a no-op.

	reportID, existing, err := ensureReport(ctx, tx, ReportDomainOfficer, report)
	if err != nil {
		return DetailLoadResult{}, fmt.Errorf("load officer report: %w", err)
	}
	counts := loadCounts{Reports: 1}
	if existing {
		counts.ExistingRows++
	}
	for _, subject := range subjects {
		loaded, err := loadOfficerSubject(ctx, tx, reportID, subject)
		if err != nil {
			return DetailLoadResult{}, err
		}
		counts.add(loaded)
	}
	if err := tx.Commit(ctx); err != nil {
		return DetailLoadResult{}, fmt.Errorf("commit officer report: %w", err)
	}
	return detailResult(counts), nil
}

func loadOfficerSubject(ctx context.Context, tx pgx.Tx, reportID int64, subject OfficerReportSubject) (loadCounts, error) {
	personID, err := ensurePerson(ctx, tx, subject.Summary.Name)
	if err != nil {
		return loadCounts{}, fmt.Errorf("resolve officer %q %q: %w", subject.Summary.Name.FirstName, subject.Summary.Name.LastName, err)
	}
	var subjectID int64
	var inserted bool
	err = tx.QueryRow(ctx, `
INSERT INTO report_subjects (
    report_id, person_id, subject_kind, caucus, role, salaries, travel_total,
    hospitality_total, contract_total, source_url, travel_source_url,
    hospitality_source_url, contract_source_url, travel_csv_url,
    hospitality_csv_url, contract_csv_url
)
VALUES ($1, $2, 'officer', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (report_id, person_id) DO UPDATE SET
    caucus = EXCLUDED.caucus,
    role = EXCLUDED.role,
    salaries = EXCLUDED.salaries,
    travel_total = EXCLUDED.travel_total,
    hospitality_total = EXCLUDED.hospitality_total,
    contract_total = EXCLUDED.contract_total,
    source_url = EXCLUDED.source_url,
    travel_source_url = EXCLUDED.travel_source_url,
    hospitality_source_url = EXCLUDED.hospitality_source_url,
    contract_source_url = EXCLUDED.contract_source_url,
    travel_csv_url = EXCLUDED.travel_csv_url,
    hospitality_csv_url = EXCLUDED.hospitality_csv_url,
    contract_csv_url = EXCLUDED.contract_csv_url,
    updated_at = CURRENT_TIMESTAMP
RETURNING id, (xmax = 0)`,
		reportID, personID, sourceText(subject.Summary.Caucus), sourceText(subject.Summary.Role), subject.Summary.Salaries,
		subject.Summary.TravelExpenses.TotalAmount, subject.Summary.HospitalityExpenses.TotalAmount,
		subject.Summary.ContractExpenses.TotalAmount, sourceText(subject.Summary.URL),
		sourceText(subject.Expenses.TravelExpensesURL), sourceText(subject.Expenses.HospitalityExpensesURL), sourceText(subject.Expenses.ContractExpensesURL),
		sourceText(subject.Expenses.TravelExpensesCSVURL), sourceText(subject.Expenses.HospitalityExpensesCSVURL), sourceText(subject.Expenses.ContractExpensesCSVURL)).Scan(&subjectID, &inserted)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert officer report subject: %w", err)
	}
	counts := loadCounts{Subjects: 1}
	if !inserted {
		counts.ExistingRows++
	}
	contractURL, err := expenseSourceURL(subject.Expenses.ContractExpensesURL, "officer contract expense")
	if err != nil && len(subject.Expenses.ContractExpenses) > 0 {
		return loadCounts{}, err
	}
	for _, expense := range subject.Expenses.ContractExpenses {
		loaded, err := loadOfficerContract(ctx, tx, subjectID, contractURL, expense)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	hospitalityURL, err := expenseSourceURL(subject.Expenses.HospitalityExpensesURL, "officer hospitality expense")
	if err != nil && len(subject.Expenses.HospitalityExpenses) > 0 {
		return loadCounts{}, err
	}
	for _, expense := range subject.Expenses.HospitalityExpenses {
		loaded, err := loadOfficerHospitality(ctx, tx, subjectID, hospitalityURL, expense)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	travelURL, err := expenseSourceURL(subject.Expenses.TravelExpensesURL, "officer travel expense")
	if err != nil && len(subject.Expenses.TravelExpenses) > 0 {
		return loadCounts{}, err
	}
	for _, expense := range subject.Expenses.TravelExpenses {
		loaded, err := loadOfficerTravel(ctx, tx, subjectID, travelURL, expense)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	return counts, nil
}

func loadOfficerContract(ctx context.Context, tx pgx.Tx, subjectID int64, sourceURL string, expense types.OfficerContractExpense) (loadCounts, error) {
	key, err := sourceKey("officer-contract", expense)
	if err != nil {
		return loadCounts{}, err
	}
	date, rawDate := sourceDate(expense.Date)
	description := sourceText(expense.Description)
	_, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO officer_contract_expenses (report_subject_id, source_key, source_url, supplier, description, category, expense_date, source_date, total_cost)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (report_subject_id, source_key) DO NOTHING RETURNING id`,
		`SELECT id FROM officer_contract_expenses WHERE report_subject_id = $1 AND source_key = $2`, subjectID, key,
		sourceURL, sourceText(expense.Supplier), description, contractCategory(description), date, rawDate, expense.TotalCost)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert officer contract: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	return counts, nil
}

func loadOfficerHospitality(ctx context.Context, tx pgx.Tx, subjectID int64, sourceURL string, expense types.OfficerHospitalityExpense) (loadCounts, error) {
	key, err := sourceKey("officer-hospitality", expense)
	if err != nil {
		return loadCounts{}, err
	}
	date, rawDate := sourceDate(expense.Date)
	id, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO officer_hospitality_expenses (report_subject_id, source_key, source_url, expense_date, source_date, location, total_attendance, purpose, total_cost, cost_per_person)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (report_subject_id, source_key) DO NOTHING RETURNING id`,
		`SELECT id FROM officer_hospitality_expenses WHERE report_subject_id = $1 AND source_key = $2`, subjectID, key,
		sourceURL, date, rawDate, sourceText(expense.Location), expense.TotalAttendance, sourceText(expense.Purpose), expense.TotalCost, expense.CostPerPerson)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert officer hospitality: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for _, supplier := range expense.Suppliers {
		name := sourceText(supplier.Name)
		if name == "" {
			continue
		}
		tag, err := tx.Exec(ctx, `
INSERT INTO officer_hospitality_suppliers (hospitality_expense_id, supplier, cost)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING`, id, name, supplier.Cost)
		if err != nil {
			return loadCounts{}, fmt.Errorf("insert officer hospitality supplier: %w", err)
		}
		counts.ChildRecords += int(tag.RowsAffected())
	}
	return counts, nil
}

func loadOfficerTravel(ctx context.Context, tx pgx.Tx, subjectID int64, sourceURL string, expense types.OfficerTravelExpense) (loadCounts, error) {
	key, err := sourceKey("officer-travel", expense)
	if err != nil {
		return loadCounts{}, err
	}
	travellerID, err := ensureOptionalPerson(ctx, tx, expense.Traveller)
	if err != nil {
		return loadCounts{}, fmt.Errorf("resolve officer travel traveller: %w", err)
	}
	start, end, sourceStart, sourceEnd := sourceDateRange(expense.Period)
	id, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO officer_travel_expenses (report_subject_id, source_key, source_url, period_start, period_end, source_period_start, source_period_end, departure_city, transportation_cost, accommodation_cost, meals_and_incidentals_cost, total_cost, purpose, traveller_person_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (report_subject_id, source_key) DO NOTHING RETURNING id`,
		`SELECT id FROM officer_travel_expenses WHERE report_subject_id = $1 AND source_key = $2`, subjectID, key,
		sourceURL, start, end, sourceStart, sourceEnd, sourceText(expense.DepartureCity), expense.TransportationCost,
		expense.AccommodationCost, expense.MealsAndIncidentals, expense.TotalCost, sourceText(expense.Purpose), travellerID)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert officer travel: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for position, place := range uniqueOrderedStrings(expense.PlacesVisited) {
		tag, err := tx.Exec(ctx, `INSERT INTO officer_travel_places (travel_expense_id, position, place) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, position, place)
		if err != nil {
			return loadCounts{}, fmt.Errorf("insert officer travel place: %w", err)
		}
		counts.ChildRecords += int(tag.RowsAffected())
	}
	return counts, nil
}
