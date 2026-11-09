package main

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/willondrik/hoc-spend/pkg/scraper/load"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestAuditMPParsed(t *testing.T) {
	subject := load.MPReportSubject{
		Summary: types.MPExpenseSummary{
			ContractExpenses:    types.ExpenseCategory{TotalAmount: decimal.RequireFromString("10.00")},
			HospitalityExpenses: types.ExpenseCategory{TotalAmount: decimal.RequireFromString("20.00")},
			TravelExpenses:      types.ExpenseCategory{TotalAmount: decimal.RequireFromString("30.00")},
		},
		Expenses: types.MPExpenses{
			ContractExpenses:    []*types.ContractExpense{{Total: decimal.RequireFromString("10.00")}},
			HospitalityExpenses: []*types.HospitalityExpense{{TotalCost: decimal.RequireFromString("20.00")}},
			TravelExpenses:      []*types.TravelExpense{{Costs: types.TravelCosts{Total: decimal.RequireFromString("30.00")}}},
		},
	}

	report := types.ExpenditureReport{FiscalYear: 2026, FiscalQuarter: 1}
	if got := auditMPParsed(report, []load.MPReportSubject{subject}); got.Checked != 3 || got.Failed != 0 {
		t.Fatalf("auditMPParsed() = %+v, want 3 passing checks", got)
	}
	subject.Summary.TravelExpenses.TotalAmount = decimal.RequireFromString("31.00")
	if got := auditMPParsed(report, []load.MPReportSubject{subject}); got.Checked != 3 || got.Failed != 1 {
		t.Fatalf("auditMPParsed() mismatch = %+v, want one failed check", got)
	} else if len(got.Details) != 1 || !strings.Contains(got.Details[0], "FY2026 Q1") {
		t.Fatalf("auditMPParsed() mismatch details = %#v, want fiscal-period context", got.Details)
	}
}

func TestAuditOfficerParsed(t *testing.T) {
	subject := load.OfficerReportSubject{
		Summary: types.OfficerExpenseSummary{
			ContractExpenses:    types.ExpenseCategory{TotalAmount: decimal.RequireFromString("10.00")},
			HospitalityExpenses: types.ExpenseCategory{TotalAmount: decimal.RequireFromString("20.00")},
			TravelExpenses:      types.ExpenseCategory{TotalAmount: decimal.RequireFromString("30.00")},
		},
		Expenses: types.OfficerExpenses{
			ContractExpenses:    []types.OfficerContractExpense{{TotalCost: decimal.RequireFromString("10.00")}},
			HospitalityExpenses: []types.OfficerHospitalityExpense{{TotalCost: decimal.RequireFromString("20.00")}},
			TravelExpenses:      []types.OfficerTravelExpense{{TotalCost: decimal.RequireFromString("30.00")}},
		},
	}

	report := types.ExpenditureReport{FiscalYear: 2026, FiscalQuarter: 1}
	if got := auditOfficerParsed(report, []load.OfficerReportSubject{subject}); got.Checked != 3 || got.Failed != 0 {
		t.Fatalf("auditOfficerParsed() = %+v, want 3 passing checks", got)
	}
	subject.Summary.ContractExpenses.TotalAmount = decimal.RequireFromString("9.99")
	if got := auditOfficerParsed(report, []load.OfficerReportSubject{subject}); got.Checked != 3 || got.Failed != 1 {
		t.Fatalf("auditOfficerParsed() mismatch = %+v, want one failed check", got)
	}
}
