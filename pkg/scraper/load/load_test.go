package load

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestUniqueMPProfiles(t *testing.T) {
	profiles := []*types.MPDetails{
		{Name: types.PersonName{FirstName: "Ada", LastName: "Lovelace"}, ProfileURL: " https://example.test/members/1 "},
		{Name: types.PersonName{FirstName: "Ada", LastName: "Lovelace"}, ProfileURL: "https://example.test/members/1"},
		{Name: types.PersonName{FirstName: "Grace", LastName: "Hopper"}, ProfileURL: "https://example.test/members/2"},
	}

	got, duplicates, err := uniqueMPProfiles(profiles)
	if err != nil {
		t.Fatalf("uniqueMPProfiles() error = %v", err)
	}
	if duplicates != 1 {
		t.Fatalf("duplicate count = %d, want 1", duplicates)
	}
	if len(got) != 2 {
		t.Fatalf("unique profile count = %d, want 2", len(got))
	}
	if got[0].ProfileURL != "https://example.test/members/1" || got[1].ProfileURL != "https://example.test/members/2" {
		t.Fatalf("profile URLs = [%q, %q], want sorted trimmed URLs", got[0].ProfileURL, got[1].ProfileURL)
	}
}

func TestUniqueMPProfilesRejectsMissingStableIdentity(t *testing.T) {
	_, _, err := uniqueMPProfiles([]*types.MPDetails{{Name: types.PersonName{FirstName: "Ada"}}})
	if err == nil {
		t.Fatal("uniqueMPProfiles() error = nil, want error for empty profile URL")
	}
}

func TestFilterNewMPProfiles(t *testing.T) {
	profiles := []*types.MPDetails{
		{ProfileURL: "https://example.test/members/1"},
		{ProfileURL: "https://example.test/members/2"},
	}

	got := filterNewMPProfiles(profiles, map[string]struct{}{
		"https://example.test/members/1": {},
	})
	if len(got) != 1 || got[0].ProfileURL != "https://example.test/members/2" {
		t.Fatalf("filterNewMPProfiles() = %#v, want only profile 2", got)
	}
}

func TestUniqueReports(t *testing.T) {
	reports := []types.ExpenditureReport{
		{FiscalYear: 2025, FiscalQuarter: 1, URL: " https://example.test/q1 "},
		{FiscalYear: 2025, FiscalQuarter: 1, URL: "https://example.test/q1-republished"},
		{FiscalYear: 2025, FiscalQuarter: 2, URL: "https://example.test/q2"},
	}

	got, duplicates, err := uniqueReports(reports)
	if err != nil {
		t.Fatalf("uniqueReports() error = %v", err)
	}
	if duplicates != 1 {
		t.Fatalf("duplicate count = %d, want 1", duplicates)
	}
	if len(got) != 2 {
		t.Fatalf("unique report count = %d, want 2", len(got))
	}
}

func TestFilterNewReportsMatchesBothDatabaseKeys(t *testing.T) {
	reports := []types.ExpenditureReport{
		{FiscalYear: 2025, FiscalQuarter: 1, URL: "https://example.test/q1-new-url"},
		{FiscalYear: 2025, FiscalQuarter: 2, URL: "https://example.test/q2"},
		{FiscalYear: 2025, FiscalQuarter: 3, URL: "https://example.test/q3"},
	}

	got := filterNewReports(reports, existingReports{
		urls: map[string]struct{}{
			"https://example.test/q2": {},
		},
		periods: map[string]struct{}{
			reportPeriodKey(2025, 1): {},
		},
	})
	want := []types.ExpenditureReport{{FiscalYear: 2025, FiscalQuarter: 3, URL: "https://example.test/q3"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filterNewReports() = %#v, want %#v", got, want)
	}
}

func TestFilterNewReportsDoesNotTreatAnotherDomainPeriodAsExisting(t *testing.T) {
	reports := []types.ExpenditureReport{
		{FiscalYear: 2025, FiscalQuarter: 1, URL: "https://example.test/mp-q1"},
	}

	got := filterNewReports(reports, existingReports{
		urls: map[string]struct{}{
			"https://example.test/officer-q1": {},
		},
		periods: map[string]struct{}{},
	})
	if !reflect.DeepEqual(got, reports) {
		t.Fatalf("filterNewReports() = %#v, want %#v", got, reports)
	}
}

func TestParseReportDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *time.Time
		fail  bool
	}{
		{name: "ISO date", input: "2025-04-01", want: timePtr(time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC))},
		{name: "empty", input: "", want: nil},
		{name: "not provided", input: "Not Provided", want: nil},
		{name: "invalid", input: "April 1, 2025", fail: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseReportDate(test.input)
			if test.fail {
				if err == nil {
					t.Fatal("parseReportDate() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseReportDate() error = %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("parseReportDate() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSourceKeyIsDeterministicAndNamespaced(t *testing.T) {
	value := struct {
		Supplier string
		Total    int
	}{Supplier: "Example Supplier", Total: 125}

	first, err := sourceKey("mp-contract", value)
	if err != nil {
		t.Fatalf("first sourceKey() error = %v", err)
	}
	second, err := sourceKey("mp-contract", value)
	if err != nil {
		t.Fatalf("second sourceKey() error = %v", err)
	}
	otherNamespace, err := sourceKey("officer-contract", value)
	if err != nil {
		t.Fatalf("namespaced sourceKey() error = %v", err)
	}
	if first != second {
		t.Fatalf("sourceKey() = %q then %q, want deterministic result", first, second)
	}
	if first == otherNamespace {
		t.Fatal("sourceKey() should differ for different namespaces")
	}
	if len(first) != 64 {
		t.Fatalf("sourceKey() length = %d, want SHA-256 hex length 64", len(first))
	}
}

func TestOccurrenceSourceKeyPreservesFirstAndSeparatesDuplicates(t *testing.T) {
	baseKey := "base-key"
	first, err := occurrenceSourceKey(baseKey, 1)
	if err != nil {
		t.Fatalf("first occurrenceSourceKey() error = %v", err)
	}
	if first != baseKey {
		t.Fatalf("first occurrence key = %q, want base key %q", first, baseKey)
	}
	second, err := occurrenceSourceKey(baseKey, 2)
	if err != nil {
		t.Fatalf("second occurrenceSourceKey() error = %v", err)
	}
	if second == baseKey || len(second) != 64 {
		t.Fatalf("second occurrence key = %q, want distinct SHA-256 key", second)
	}
	repeatedSecond, err := occurrenceSourceKey(baseKey, 2)
	if err != nil {
		t.Fatalf("repeated second occurrenceSourceKey() error = %v", err)
	}
	if repeatedSecond != second {
		t.Fatalf("second occurrence key = %q then %q, want deterministic result", second, repeatedSecond)
	}
	if _, err := occurrenceSourceKey(baseKey, 0); err == nil {
		t.Fatal("occurrenceSourceKey() error = nil for zero occurrence")
	}
}

func TestOccurrenceSourceKeySeparatesRepeatedAdministrationSourceRows(t *testing.T) {
	contract := types.AdminContractExpense{
		Suppliers:             []string{"Example Supplier"},
		Description:           "Example contract",
		ContractPeriod:        types.DateRange{StartDate: "2026-04-01", EndDate: "2026-06-30"},
		OriginalContractValue: decimal.RequireFromString("100.00"),
		AmendedContractValue:  decimal.RequireFromString("100.00"),
	}
	baseKey, err := sourceKey("administration-contract", contract)
	if err != nil {
		t.Fatalf("sourceKey() error = %v", err)
	}
	first, err := occurrenceSourceKey(baseKey, 1)
	if err != nil {
		t.Fatalf("first occurrenceSourceKey() error = %v", err)
	}
	second, err := occurrenceSourceKey(baseKey, 2)
	if err != nil {
		t.Fatalf("second occurrenceSourceKey() error = %v", err)
	}
	if first == second {
		t.Fatal("repeated Administration contract rows received the same source key")
	}

	travel := types.AdminTravelExpense{EventID: "reused-event-id", Purpose: "Example travel", TotalCost: decimal.RequireFromString("100.00")}
	travelKey, err := sourceKey("administration-travel", travel)
	if err != nil {
		t.Fatalf("travel sourceKey() error = %v", err)
	}
	travelFirst, err := occurrenceSourceKey(travelKey, 1)
	if err != nil {
		t.Fatalf("first travel occurrenceSourceKey() error = %v", err)
	}
	travelSecond, err := occurrenceSourceKey(travelKey, 2)
	if err != nil {
		t.Fatalf("second travel occurrenceSourceKey() error = %v", err)
	}
	if travelFirst == travelSecond {
		t.Fatal("repeated Administration travel rows received the same source key")
	}
}

func TestSourceDatePreservesRawValue(t *testing.T) {
	parsed, raw := sourceDate(" Monday, April 1, 2024 ")
	if raw != "Monday, April 1, 2024" {
		t.Fatalf("raw date = %q, want trimmed source text", raw)
	}
	if parsed == nil || parsed.Format("2006-01-02") != "2024-04-01" {
		t.Fatalf("parsed date = %v, want 2024-04-01", parsed)
	}

	parsed, raw = sourceDate("Source format unavailable")
	if parsed != nil || raw != "Source format unavailable" {
		t.Fatalf("unparseable date = (%v, %q), want (nil, source text)", parsed, raw)
	}
}

func TestSourceDateRangeNormalizesReversedParsedDatesWithoutChangingSourceText(t *testing.T) {
	start, end, sourceStart, sourceEnd := sourceDateRange(types.DateRange{
		StartDate: "June 30, 2026",
		EndDate:   "April 1, 2026",
	})
	if sourceStart != "June 30, 2026" || sourceEnd != "April 1, 2026" {
		t.Fatalf("source text = (%q, %q), want original source ordering", sourceStart, sourceEnd)
	}
	if start == nil || end == nil || start.Format("2006-01-02") != "2026-04-01" || end.Format("2006-01-02") != "2026-06-30" {
		t.Fatalf("normalized range = (%v, %v), want (2026-04-01, 2026-06-30)", start, end)
	}
}

func TestOptionalPersonAcceptsBlankNameWithoutDatabase(t *testing.T) {
	if sourceText(types.PersonName{}.FirstName) != "" || sourceText(types.PersonName{}.LastName) != "" {
		t.Fatal("blank person name should remain blank before optional resolution")
	}
}

func TestAdministrationReportCategoryURLsAreTrimmedBeforePersistence(t *testing.T) {
	report := types.AdminReport{
		Travel:      types.AdminTravelExpenseCollection{URL: " https://example.test/admin/travel "},
		Hospitality: types.AdminHospitalityExpenseCollection{URL: "\thttps://example.test/admin/hospitality\n"},
		Contract:    types.AdminContractExpenseCollection{URL: " https://example.test/admin/contracts\r\n"},
	}

	if got, want := sourceText(report.Travel.URL), "https://example.test/admin/travel"; got != want {
		t.Fatalf("travel source URL = %q, want %q", got, want)
	}
	if got, want := sourceText(report.Hospitality.URL), "https://example.test/admin/hospitality"; got != want {
		t.Fatalf("hospitality source URL = %q, want %q", got, want)
	}
	if got, want := sourceText(report.Contract.URL), "https://example.test/admin/contracts"; got != want {
		t.Fatalf("contract source URL = %q, want %q", got, want)
	}
}

func TestExpenseSourceURL(t *testing.T) {
	url, err := expenseSourceURL(" https://example.test/source ", "MP contract expense")
	if err != nil {
		t.Fatalf("expenseSourceURL() error = %v", err)
	}
	if url != "https://example.test/source" {
		t.Fatalf("expenseSourceURL() = %q, want trimmed URL", url)
	}

	if _, err := expenseSourceURL(" \t\n", "MP contract expense"); err == nil {
		t.Fatal("expenseSourceURL() error = nil, want missing source URL error")
	}
}

func TestPersonLockIdentityContainsNoNULByte(t *testing.T) {
	identity := strings.ToLower("Ada") + "\x1f" + strings.ToLower("Lovelace")
	if strings.ContainsRune(identity, '\x00') {
		t.Fatalf("advisory-lock identity %q contains a PostgreSQL-incompatible NUL byte", identity)
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
