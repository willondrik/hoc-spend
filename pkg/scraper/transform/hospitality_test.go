package transform

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestHospitalityCostPerPerson(t *testing.T) {
	t.Run("MP", func(t *testing.T) {
		doc := hospitalityDocument(t, `
			<table><tbody>
				<tr class="expenses-main-info"><td>2026-01-01</td><td>Ottawa</td><td>3</td><td>Meeting</td><td>$100.00</td></tr>
				<tr><td><div class="col-md-4">Reception</div><table><tbody></tbody></table></td></tr>
			</tbody></table>`)

		expenses, err := MpHospitalityExpenses(doc)
		if err != nil {
			t.Fatalf("MpHospitalityExpenses returned an error: %v", err)
		}
		if len(expenses) != 1 || expenses[0].CostPerPerson.StringFixed(2) != "33.33" {
			t.Fatalf("unexpected MP cost per person: %#v", expenses)
		}
	})

	t.Run("officer", func(t *testing.T) {
		doc := hospitalityDocument(t, `
			<table><tbody>
				<tr><td>2026-01-01</td><td>Ottawa</td><td>4</td><td>Meeting</td><td>$100.00</td></tr>
				<tr><td><table><tbody></tbody></table></td></tr>
			</tbody></table>`)

		expenses, err := OfficerHospitalityExpenses(doc)
		if err != nil {
			t.Fatalf("OfficerHospitalityExpenses returned an error: %v", err)
		}
		if len(expenses) != 1 || expenses[0].CostPerPerson.StringFixed(2) != "25.00" {
			t.Fatalf("unexpected officer cost per person: %#v", expenses)
		}
	})

	t.Run("administration", func(t *testing.T) {
		doc := hospitalityDocument(t, `
			<table class="table expenses table-striped" summary="Hospitality Events"><tbody>
				<tr><td>1</td><td>Host</td><td>Meeting</td><td>2026-01-01</td><td>$99.99</td></tr>
				<tr><td><p aria-labelledby="TotalAttendees">3</p></td></tr>
			</tbody></table>`)

		expenses, err := AdminHospitalityExpenses(doc)
		if err != nil {
			t.Fatalf("AdminHospitalityExpenses returned an error: %v", err)
		}
		if len(expenses) != 1 || expenses[0].CostPerPerson.StringFixed(2) != "33.33" {
			t.Fatalf("unexpected administration cost per person: %#v", expenses)
		}
	})
}

func TestHospitalityCostPerPersonWithoutAttendance(t *testing.T) {
	doc := hospitalityDocument(t, `
		<table><tbody>
			<tr class="expenses-main-info"><td>2026-01-01</td><td>Ottawa</td><td></td><td>Meeting</td><td>$100.00</td></tr>
			<tr><td><div class="col-md-4">Reception</div></td></tr>
		</tbody></table>`)

	expenses, err := MpHospitalityExpenses(doc)
	if err != nil {
		t.Fatalf("MpHospitalityExpenses returned an error: %v", err)
	}
	if len(expenses) != 1 || !expenses[0].CostPerPerson.IsZero() {
		t.Fatalf("cost per person should be zero when attendance is missing: %#v", expenses)
	}
}

func hospitalityDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("create HTML document: %v", err)
	}
	return doc
}
