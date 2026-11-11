package load

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

// MPReportSubject joins a quarterly MP summary/details payload to the durable
// House of Commons profile URL for that MP. The profile must have been loaded
// before report details are persisted.
type MPReportSubject struct {
	ProfileURL string
	Summary    types.MPExpenseSummary
	Expenses   types.MPExpenses
}

// ReuseExistingMPSubjectProfiles replaces generated source identities with an
// identity already attached to the same report/name/riding. This protects
// repeated historical loads when a House report URL changes while retaining
// source-scoped identities for previously unseen subjects.
func (l *Loader) ReuseExistingMPSubjectProfiles(ctx context.Context, report types.ExpenditureReport, subjects []MPReportSubject) error {
	for index := range subjects {
		subject := &subjects[index]
		var profileURL string
		err := l.pool.QueryRow(ctx, `
SELECT mps.profile_url
FROM report_subjects
JOIN disclosure_reports ON disclosure_reports.id = report_subjects.report_id
JOIN mp_affiliations ON mp_affiliations.id = report_subjects.mp_affiliation_id
JOIN mps ON mps.id = mp_affiliations.mp_id
JOIN people ON people.id = mps.person_id
WHERE disclosure_reports.domain = 'mp'
  AND disclosure_reports.fiscal_year = $1
  AND disclosure_reports.fiscal_quarter = $2
  AND lower(people.first_name) = lower($3)
  AND lower(people.last_name) = lower($4)
  AND mp_affiliations.constituency = $5
ORDER BY report_subjects.id DESC
LIMIT 1`, report.FiscalYear, report.FiscalQuarter,
			strings.TrimSpace(subject.Summary.Name.FirstName), strings.TrimSpace(subject.Summary.Name.LastName), strings.TrimSpace(subject.Summary.Constituency)).Scan(&profileURL)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return fmt.Errorf("reuse MP profile for %q %q: %w", subject.Summary.Name.FirstName, subject.Summary.Name.LastName, err)
		}
		subject.ProfileURL = profileURL
	}
	return nil
}

// LoadMPReport writes an MP quarterly report and all supplied MP details in
// one transaction. It refuses name-only MP matching: every subject must supply
// an existing durable profile URL.
func (l *Loader) LoadMPReport(ctx context.Context, report types.ExpenditureReport, subjects []MPReportSubject) (DetailLoadResult, error) {
	tx, err := beginLoad(ctx, l)
	if err != nil {
		return DetailLoadResult{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // Commit makes this a no-op.

	reportID, existing, err := ensureReport(ctx, tx, ReportDomainMP, report)
	if err != nil {
		return DetailLoadResult{}, fmt.Errorf("load MP report: %w", err)
	}
	counts := loadCounts{Reports: 1}
	if existing {
		counts.ExistingRows++
	}
	for _, subject := range subjects {
		loaded, err := loadMPSubject(ctx, tx, reportID, subject)
		if err != nil {
			return DetailLoadResult{}, err
		}
		counts.add(loaded)
	}
	if err := tx.Commit(ctx); err != nil {
		return DetailLoadResult{}, fmt.Errorf("commit MP report: %w", err)
	}
	return detailResult(counts), nil
}

func loadMPSubject(ctx context.Context, tx pgx.Tx, reportID int64, subject MPReportSubject) (loadCounts, error) {
	profileURL := sourceText(subject.ProfileURL)
	if profileURL == "" {
		return loadCounts{}, fmt.Errorf("MP report subject profile URL is required")
	}
	var mpID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM mps WHERE profile_url = $1`, profileURL).Scan(&mpID); err != nil {
		if err == pgx.ErrNoRows {
			return loadCounts{}, fmt.Errorf("MP profile %q has not been loaded", profileURL)
		}
		return loadCounts{}, fmt.Errorf("query MP profile %q: %w", profileURL, err)
	}

	start, end, _, _ := sourceDateRange(subject.Summary.ReportingPeriod)
	var affiliationID int64
	err := tx.QueryRow(ctx, `
INSERT INTO mp_affiliations (mp_id, constituency, caucus, effective_from, effective_to)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (mp_id, constituency, caucus) DO NOTHING RETURNING id`,
		mpID, sourceText(subject.Summary.Constituency), sourceText(subject.Summary.Caucus), start, end).Scan(&affiliationID)
	affiliationInserted := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return loadCounts{}, fmt.Errorf("insert MP affiliation: %w", err)
	}
	if !affiliationInserted {
		if err := tx.QueryRow(ctx, `
SELECT id FROM mp_affiliations
WHERE mp_id = $1 AND constituency = $2 AND caucus = $3`,
			mpID, sourceText(subject.Summary.Constituency), sourceText(subject.Summary.Caucus)).Scan(&affiliationID); err != nil {
			return loadCounts{}, fmt.Errorf("find existing MP affiliation: %w", err)
		}
	}
	var subjectID int64
	var subjectInserted bool
	err = tx.QueryRow(ctx, `
INSERT INTO report_subjects (
    report_id, mp_affiliation_id, subject_kind, caucus, salaries, travel_total,
    hospitality_total, contract_total, source_url, travel_source_url,
    hospitality_source_url, contract_source_url, travel_csv_url,
    hospitality_csv_url, contract_csv_url
)
VALUES ($1, $2, 'mp', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
ON CONFLICT (report_id, mp_affiliation_id) DO UPDATE SET
    caucus = EXCLUDED.caucus,
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
RETURNING id, (xmax = 0)`, reportID, affiliationID,
		sourceText(subject.Summary.Caucus), subject.Summary.Salaries, subject.Summary.TravelExpenses.TotalAmount,
		subject.Summary.HospitalityExpenses.TotalAmount, subject.Summary.ContractExpenses.TotalAmount,
		sourceText(subject.Summary.URL), sourceText(subject.Expenses.TravelExpensesURL),
		sourceText(subject.Expenses.HospitalityExpensesURL), sourceText(subject.Expenses.ContractExpensesURL),
		sourceText(subject.Expenses.TravelExpensesCSVURL), sourceText(subject.Expenses.HospitalityExpensesCSVURL), sourceText(subject.Expenses.ContractExpensesCSVURL)).Scan(&subjectID, &subjectInserted)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert MP report subject: %w", err)
	}
	counts := loadCounts{Subjects: 1}
	if !affiliationInserted {
		counts.ExistingRows++
	}
	if !subjectInserted {
		counts.ExistingRows++
	}
	contractOccurrences := make(map[string]int, len(subject.Expenses.ContractExpenses))
	contractURL, err := expenseSourceURL(subject.Expenses.ContractExpensesURL, "MP contract expense")
	if err != nil && len(subject.Expenses.ContractExpenses) > 0 {
		return loadCounts{}, err
	}
	for _, expense := range subject.Expenses.ContractExpenses {
		if expense == nil {
			return loadCounts{}, fmt.Errorf("MP contract expense is nil")
		}
		baseKey, err := sourceKey("mp-contract", *expense)
		if err != nil {
			return loadCounts{}, fmt.Errorf("create MP contract source key: %w", err)
		}
		contractOccurrences[baseKey]++
		key, err := occurrenceSourceKey(baseKey, contractOccurrences[baseKey])
		if err != nil {
			return loadCounts{}, fmt.Errorf("create MP contract occurrence key: %w", err)
		}
		loaded, err := loadMPContract(ctx, tx, subjectID, key, contractURL, *expense)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	hospitalityURL, err := expenseSourceURL(subject.Expenses.HospitalityExpensesURL, "MP hospitality expense")
	if err != nil && len(subject.Expenses.HospitalityExpenses) > 0 {
		return loadCounts{}, err
	}
	for _, expense := range subject.Expenses.HospitalityExpenses {
		if expense == nil {
			return loadCounts{}, fmt.Errorf("MP hospitality expense is nil")
		}
		loaded, err := loadMPHospitality(ctx, tx, subjectID, hospitalityURL, *expense)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	travelURL, err := expenseSourceURL(subject.Expenses.TravelExpensesURL, "MP travel expense")
	if err != nil && len(subject.Expenses.TravelExpenses) > 0 {
		return loadCounts{}, err
	}
	for _, expense := range subject.Expenses.TravelExpenses {
		if expense == nil {
			return loadCounts{}, fmt.Errorf("MP travel expense is nil")
		}
		loaded, err := loadMPTravel(ctx, tx, subjectID, travelURL, *expense)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	return counts, nil
}

func loadMPContract(ctx context.Context, tx pgx.Tx, subjectID int64, key, sourceURL string, expense types.ContractExpense) (loadCounts, error) {
	date, rawDate := sourceDate(expense.Date)
	description := sourceText(expense.Description)
	_, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO mp_contract_expenses (report_subject_id, source_key, source_url, supplier, description, category, expense_date, source_date, total, is_refunded)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (report_subject_id, source_key) DO NOTHING RETURNING id`,
		`SELECT id FROM mp_contract_expenses WHERE report_subject_id = $1 AND source_key = $2`, subjectID, key,
		sourceURL, sourceText(expense.Supplier), description, contractCategory(description), date, rawDate, expense.Total, expense.IsRefunded)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert MP contract: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	return counts, nil
}

func loadMPHospitality(ctx context.Context, tx pgx.Tx, subjectID int64, sourceURL string, expense types.HospitalityExpense) (loadCounts, error) {
	key, err := sourceKey("mp-hospitality", expense)
	if err != nil {
		return loadCounts{}, err
	}
	date, rawDate := sourceDate(expense.Date)
	id, inserted, err := insertOrFindID(ctx, tx, `
INSERT INTO mp_hospitality_expenses (report_subject_id, source_key, source_url, expense_date, source_date, location, attendance, purpose, event_type, total_cost, cost_per_person, is_refunded)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
ON CONFLICT (report_subject_id, source_key) DO NOTHING RETURNING id`,
		`SELECT id FROM mp_hospitality_expenses WHERE report_subject_id = $1 AND source_key = $2`, subjectID, key,
		sourceURL, date, rawDate, sourceText(expense.Location), expense.Attendance, sourceText(expense.Purpose), sourceText(expense.Event.Type), expense.TotalCost, expense.CostPerPerson, expense.IsRefunded)
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert MP hospitality: %w", err)
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for _, item := range expense.Event.ExpenseItems {
		itemKey, err := sourceKey("mp-hospitality-item", item)
		if err != nil {
			return loadCounts{}, err
		}
		tag, err := tx.Exec(ctx, `
INSERT INTO mp_hospitality_expense_items (hospitality_expense_id, source_key, claim, supplier, cost)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (hospitality_expense_id, source_key) DO NOTHING`, id, itemKey, sourceText(item.Claim), sourceText(item.Supplier), item.Cost)
		if err != nil {
			return loadCounts{}, fmt.Errorf("insert MP hospitality item: %w", err)
		}
		counts.ChildRecords += int(tag.RowsAffected())
	}
	return counts, nil
}

func loadMPTravel(ctx context.Context, tx pgx.Tx, subjectID int64, sourceURL string, expense types.TravelExpense) (loadCounts, error) {
	start, end, sourceStart, sourceEnd := sourceDateRange(expense.DateRange)
	var id int64
	err := tx.QueryRow(ctx, `
INSERT INTO mp_travel_expenses (report_subject_id, source_url, claim, sequence, period_start, period_end, source_period_start, source_period_end, transportation_cost, accommodation_cost, meals_and_incidentals_cost, total_cost, regular_flight_points, special_flight_points, usa_flight_points, is_duplicate, is_refunded)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
ON CONFLICT (report_subject_id, sequence) DO NOTHING RETURNING id`,
		subjectID, sourceURL, sourceText(expense.Claim), expense.Sequence,
		start, end, sourceStart, sourceEnd, expense.Costs.Transportation, expense.Costs.Accommodation, expense.Costs.MealsAndIncidentals,
		expense.Costs.Total, expense.FlightPoints.Regular, expense.FlightPoints.Special, expense.FlightPoints.USA, expense.IsDuplicate, expense.IsRefunded).Scan(&id)
	inserted := err == nil
	if err != nil && err != pgx.ErrNoRows {
		return loadCounts{}, fmt.Errorf("insert MP travel: %w", err)
	}
	if !inserted {
		if err := tx.QueryRow(ctx, `
SELECT id FROM mp_travel_expenses
WHERE report_subject_id = $1 AND sequence = $2`, subjectID, expense.Sequence).Scan(&id); err != nil {
			return loadCounts{}, fmt.Errorf("find existing MP travel: %w", err)
		}
	}
	counts := loadCounts{Expenses: 1}
	if !inserted {
		counts.ExistingRows++
	}
	for _, traveller := range expense.Travellers {
		loaded, err := loadMPTraveller(ctx, tx, id, traveller)
		if err != nil {
			return loadCounts{}, err
		}
		counts.add(loaded)
	}
	return counts, nil
}

func loadMPTraveller(ctx context.Context, tx pgx.Tx, travelID int64, traveller types.Traveller) (loadCounts, error) {
	key, err := sourceKey("mp-traveller", traveller)
	if err != nil {
		return loadCounts{}, err
	}
	var personID any
	if sourceText(traveller.Name.FirstName) != "" || sourceText(traveller.Name.LastName) != "" {
		id, err := ensurePerson(ctx, tx, traveller.Name)
		if err != nil {
			return loadCounts{}, fmt.Errorf("resolve MP traveller: %w", err)
		}
		personID = id
	}
	date, rawDate := sourceDate(traveller.Date)
	tag, err := tx.Exec(ctx, `
INSERT INTO mp_travelers (travel_expense_id, source_key, person_id, traveller_type, purpose, travel_date, source_date, departure_city, destination_city, transportation_mode, distance, distance_unit, emissions, emissions_unit, travel_class)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (travel_expense_id, source_key) DO NOTHING`, travelID, key, personID, sourceText(traveller.Type), sourceText(traveller.Purpose), date, rawDate, sourceText(traveller.DepartureCity), sourceText(traveller.DestinationCity), sourceText(traveller.TransportationMode), traveller.TravelData.Distance, sourceText(traveller.TravelData.DistanceUnit), traveller.TravelData.Emissions, sourceText(traveller.TravelData.EmissionsUnit), sourceText(traveller.TravelData.Class))
	if err != nil {
		return loadCounts{}, fmt.Errorf("insert MP traveller: %w", err)
	}
	return loadCounts{ChildRecords: int(tag.RowsAffected())}, nil
}
