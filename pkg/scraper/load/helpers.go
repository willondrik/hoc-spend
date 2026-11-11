package load

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

type loadCounts struct {
	Reports      int
	Subjects     int
	Expenses     int
	ChildRecords int
	ExistingRows int
}

func (c *loadCounts) add(other loadCounts) {
	c.Reports += other.Reports
	c.Subjects += other.Subjects
	c.Expenses += other.Expenses
	c.ChildRecords += other.ChildRecords
	c.ExistingRows += other.ExistingRows
}

func sourceKey(namespace string, value any) (string, error) {
	payload, err := json.Marshal(struct {
		Namespace string `json:"namespace"`
		Value     any    `json:"value"`
	}{Namespace: namespace, Value: value})
	if err != nil {
		return "", fmt.Errorf("marshal source key payload: %w", err)
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}

// occurrenceSourceKey preserves a legacy/base source key for the first
// occurrence and derives a stable distinct key for repeated identical rows.
func occurrenceSourceKey(baseKey string, occurrence int) (string, error) {
	if occurrence < 1 {
		return "", fmt.Errorf("source-key occurrence must be positive")
	}
	if occurrence == 1 {
		return baseKey, nil
	}
	return sourceKey("source-key-occurrence", struct {
		BaseKey    string `json:"baseKey"`
		Occurrence int    `json:"occurrence"`
	}{BaseKey: baseKey, Occurrence: occurrence})
}

func sourceText(value string) string {
	return strings.TrimSpace(value)
}

func expenseSourceURL(value, expenseType string) (string, error) {
	url := sourceText(value)
	if url == "" {
		return "", fmt.Errorf("%s source URL is required", expenseType)
	}
	return url, nil
}

func sourceDate(value string) (*time.Time, string) {
	raw := sourceText(value)
	if raw == "" || strings.EqualFold(raw, "not provided") || strings.EqualFold(raw, "n/a") {
		return nil, raw
	}

	withoutWeekday := raw
	for _, weekday := range []string{"Monday, ", "Tuesday, ", "Wednesday, ", "Thursday, ", "Friday, ", "Saturday, ", "Sunday, "} {
		withoutWeekday = strings.TrimPrefix(withoutWeekday, weekday)
	}
	for _, layout := range []string{"2006-01-02", "January 2, 2006", "Jan 2, 2006"} {
		if parsed, err := time.Parse(layout, withoutWeekday); err == nil {
			return &parsed, raw
		}
	}
	return nil, raw
}

func sourceDateRange(value types.DateRange) (*time.Time, *time.Time, string, string) {
	start, sourceStart := sourceDate(value.StartDate)
	end, sourceEnd := sourceDate(value.EndDate)
	// Some House source rows contain a reversed but otherwise parseable period.
	// Keep both original strings for auditability, while normalizing the typed
	// values to the database's period_start <= period_end invariant.
	if start != nil && end != nil && start.After(*end) {
		start, end = end, start
	}
	return start, end, sourceStart, sourceEnd
}

func ensureReport(ctx context.Context, tx pgx.Tx, domain ReportDomain, report types.ExpenditureReport) (int64, bool, error) {
	if !domain.valid() {
		return 0, false, fmt.Errorf("unsupported report domain %q", domain)
	}
	if strings.TrimSpace(report.URL) == "" {
		return 0, false, fmt.Errorf("report source URL is required")
	}
	if report.FiscalYear < 1867 || report.FiscalYear > 9999 || report.FiscalQuarter < 1 || report.FiscalQuarter > 4 {
		return 0, false, fmt.Errorf("invalid fiscal period FY %d Q%d", report.FiscalYear, report.FiscalQuarter)
	}

	var id int64
	err := tx.QueryRow(ctx, `
SELECT id FROM disclosure_reports
WHERE domain = $1 AND fiscal_year = $2 AND fiscal_quarter = $3`, domain, report.FiscalYear, report.FiscalQuarter).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != pgx.ErrNoRows {
		return 0, false, fmt.Errorf("query report: %w", err)
	}

	periodStart, periodEnd, _, _ := sourceDateRange(report.DateRange)
	err = tx.QueryRow(ctx, `
INSERT INTO disclosure_reports (domain, fiscal_year, fiscal_quarter, period_start, period_end, source_url)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING
RETURNING id`, domain, report.FiscalYear, report.FiscalQuarter, periodStart, periodEnd, strings.TrimSpace(report.URL)).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if err != pgx.ErrNoRows {
		return 0, false, fmt.Errorf("insert report: %w", err)
	}

	err = tx.QueryRow(ctx, `
SELECT id FROM disclosure_reports
WHERE domain = $1 AND fiscal_year = $2 AND fiscal_quarter = $3`, domain, report.FiscalYear, report.FiscalQuarter).Scan(&id)
	if err != nil {
		return 0, false, fmt.Errorf("retrieve concurrent report insert: %w", err)
	}
	return id, true, nil
}

func ensurePerson(ctx context.Context, tx pgx.Tx, name types.PersonName) (int64, error) {
	firstName := sourceText(name.FirstName)
	lastName := sourceText(name.LastName)
	if firstName == "" && lastName == "" {
		return 0, fmt.Errorf("person name is required")
	}

	// PostgreSQL text values cannot contain NUL bytes, so use a control-byte
	// separator that cannot be produced by normal trimmed source names.
	identity := strings.ToLower(firstName) + "\x1f" + strings.ToLower(lastName)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identity); err != nil {
		return 0, fmt.Errorf("lock person identity: %w", err)
	}

	var id int64
	err := tx.QueryRow(ctx, `
SELECT id FROM people
WHERE first_name = $1 AND last_name = $2
ORDER BY id LIMIT 1`, firstName, lastName).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return 0, fmt.Errorf("query person: %w", err)
	}

	err = tx.QueryRow(ctx, `
INSERT INTO people (first_name, last_name)
VALUES ($1, $2)
RETURNING id`, firstName, lastName).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert person: %w", err)
	}
	return id, nil
}

func ensureOptionalPerson(ctx context.Context, tx pgx.Tx, name types.PersonName) (any, error) {
	if sourceText(name.FirstName) == "" && sourceText(name.LastName) == "" {
		return nil, nil
	}
	id, err := ensurePerson(ctx, tx, name)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func beginLoad(ctx context.Context, l *Loader) (pgx.Tx, error) {
	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin load transaction: %w", err)
	}
	return tx, nil
}

func insertOrFindID(ctx context.Context, tx pgx.Tx, insertSQL, selectSQL string, args ...any) (int64, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, insertSQL, args...).Scan(&id)
	if err == nil {
		return id, true, nil
	}
	if err != pgx.ErrNoRows {
		return 0, false, err
	}
	if len(args) < 2 {
		return 0, false, fmt.Errorf("lookup requires at least two arguments")
	}
	if err := tx.QueryRow(ctx, selectSQL, args[0], args[1]).Scan(&id); err != nil {
		return 0, false, err
	}
	return id, false, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = sourceText(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueOrderedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = sourceText(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
