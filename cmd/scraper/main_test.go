package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/willondrik/hoc-spend/pkg/scraper/load"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestRunOptionsValidate(t *testing.T) {
	tests := []struct {
		name string
		opts runOptions
		want bool
	}{
		{name: "read-only all domains", opts: runOptions{domain: "all", limit: 1}, want: true},
		{name: "administration apply", opts: runOptions{domain: "administration", apply: true, limit: 1}, want: true},
		{name: "officer apply", opts: runOptions{domain: "officer", apply: true, limit: 1}, want: true},
		{name: "MP apply", opts: runOptions{domain: "mp", apply: true, limit: 1}, want: true},
		{name: "all reports audit", opts: runOptions{domain: "all", apply: true, audit: true, allReports: true}, want: true},
		{name: "MP profile enrichment", opts: runOptions{domain: "all", enrichMPProfiles: true, limit: 1}, want: true},
		{name: "audit without apply", opts: runOptions{domain: "all", audit: true, limit: 1}},
		{name: "negative limit", opts: runOptions{domain: "administration", limit: -1}},
		{name: "incomplete period", opts: runOptions{domain: "administration", fiscalYear: 2026, limit: 1}},
		{name: "invalid quarter", opts: runOptions{domain: "administration", fiscalYear: 2026, fiscalQuarter: 5, limit: 1}},
		{name: "enrichment with apply", opts: runOptions{domain: "all", apply: true, enrichMPProfiles: true, limit: 1}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.opts.validate()
			if test.want && err != nil {
				t.Fatalf("validate() error = %v", err)
			}
			if !test.want && err == nil {
				t.Fatal("validate() error = nil, want error")
			}
		})
	}
}

func TestMPSourceIdentity(t *testing.T) {
	base := types.MPExpenseSummary{Name: types.PersonName{FirstName: "Jane", LastName: "Doe"}, Constituency: "Example Riding", URL: "https://example.test/report", FiscalYear: 2026, FiscalQuarter: 1}
	got, err := mpSourceIdentity(base)
	if err != nil {
		t.Fatalf("mpSourceIdentity() error = %v", err)
	}
	if !strings.HasPrefix(got, "source://ourcommons.ca/proactivedisclosure/mp/") {
		t.Fatalf("identity = %q, want source identity URL", got)
	}
	changed := base
	changed.Constituency = "Other Riding"
	changedRiding, err := mpSourceIdentity(changed)
	if err != nil {
		t.Fatalf("mpSourceIdentity(changed riding) error = %v", err)
	}
	if got == changedRiding {
		t.Fatal("identities for different ridings must not be merged")
	}
	changedReport := base
	changedReport.URL = "https://example.test/report?summaryId=republished"
	changedReport.FiscalYear = 2025
	changedReport.FiscalQuarter = 4
	changedReportIdentity, err := mpSourceIdentity(changedReport)
	if err != nil {
		t.Fatalf("mpSourceIdentity(changed report) error = %v", err)
	}
	if got != changedReportIdentity {
		t.Fatalf("identity changed from %q to %q when only report metadata changed", got, changedReportIdentity)
	}
}

func TestFilterFiscalPeriodAndLimitReports(t *testing.T) {
	reports := []types.ExpenditureReport{
		{FiscalYear: 2026, FiscalQuarter: 1, URL: "https://example.test/2026/1"},
		{FiscalYear: 2025, FiscalQuarter: 4, URL: "https://example.test/2025/4"},
		{FiscalYear: 2025, FiscalQuarter: 3, URL: "https://example.test/2025/3"},
	}

	if got := filterFiscalPeriod(reports, 2025, 4); !reflect.DeepEqual(got, reports[1:2]) {
		t.Fatalf("filterFiscalPeriod() = %#v, want %#v", got, reports[1:2])
	}
	if got := filterFiscalPeriod(reports, 0, 0); !reflect.DeepEqual(got, reports) {
		t.Fatalf("unfiltered reports = %#v, want %#v", got, reports)
	}
	if got := limitReports(reports, 1); !reflect.DeepEqual(got, reports[:1]) {
		t.Fatalf("limitReports() = %#v, want newest report %#v", got, reports[:1])
	}
	if got := limitReports(reports, 0); len(got) != 0 {
		t.Fatalf("zero limit returned %#v, want no reports", got)
	}
}

func TestMPProfilesFromSubjectsUsesUniqueProfileURLs(t *testing.T) {
	subjects := []load.MPReportSubject{
		{SourceIdentity: " source-a ", Summary: types.MPExpenseSummary{Name: types.PersonName{FirstName: "Ada", LastName: "Lovelace"}}},
		{SourceIdentity: "source-a", Summary: types.MPExpenseSummary{Name: types.PersonName{FirstName: "Ada", LastName: "Lovelace"}}},
		{SourceIdentity: "source-b", Summary: types.MPExpenseSummary{Name: types.PersonName{FirstName: "Grace", LastName: "Hopper"}}},
	}

	profiles := mpProfilesFromSubjects(subjects)
	if len(profiles) != 2 || profiles[0].SourceIdentity != "source-a" || profiles[1].SourceIdentity != "source-b" {
		t.Fatalf("mpProfilesFromSubjects() = %#v, want unique trimmed source identities", profiles)
	}
}
