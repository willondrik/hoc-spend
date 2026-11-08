package load

import (
	"reflect"
	"testing"
	"time"

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

func TestOptionalPersonAcceptsBlankNameWithoutDatabase(t *testing.T) {
	if sourceText(types.PersonName{}.FirstName) != "" || sourceText(types.PersonName{}.LastName) != "" {
		t.Fatal("blank person name should remain blank before optional resolution")
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
