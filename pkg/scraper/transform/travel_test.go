package transform

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestMpTravelExpensesParsesEveryMainRowWhenHiddenDetailsContainMultipleRows(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<table><tbody>
			<tr class="expenses-main-info"><td>From June 2, 2025 to June 30, 2025</td><td>Summerside</td><td>Tignish</td><td>$1,138.39</td><td>$0.00</td><td>$0.00</td><td>$1,138.39</td><td></td></tr>
			<tr class="hidden"><td><table><thead><tr><th>Purpose of Travel</th><th>Traveller</th></tr></thead><tbody><tr><td>Business</td><td>Employee, Jane Doe</td></tr></tbody></table></td></tr>
			<tr class="expenses-main-info"><td>From June 14, 2025 to June 28, 2025</td><td>Tignish</td><td>Tyne Valley, Traveller's Rest</td><td>$692.19</td><td>$0.00</td><td>$227.48</td><td>$919.67</td><td></td></tr>
			<tr class="hidden"><td><table><thead><tr><th>Purpose of Travel</th><th>Traveller</th></tr></thead><tbody><tr><td>Business</td><td>Member, Jane Doe</td></tr></tbody></table></td></tr>
			<tr class="expenses-main-info"><td>From July 1, 2025 to July 18, 2025</td><td>Tignish</td><td>Summerside</td><td>$962.17</td><td>$0.00</td><td>$267.12</td><td>$1,229.29</td><td></td></tr>
			<tr class="hidden"><td><table><thead><tr><th>Purpose of Travel</th><th>Traveller</th></tr></thead><tbody><tr><td>Business</td><td>Member, Jane Doe</td></tr></tbody></table></td></tr>
		</tbody></table>`))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	expenses, err := MpTravelExpenses(doc)
	if err != nil {
		t.Fatalf("MpTravelExpenses() error = %v", err)
	}
	if len(expenses) != 3 {
		t.Fatalf("parsed travel entries = %d, want 3", len(expenses))
	}
	if got := expenses[1].Costs.Total.StringFixed(2); got != "919.67" {
		t.Fatalf("middle travel total = %s, want 919.67", got)
	}
	for index, expense := range expenses {
		if expense.Sequence != index {
			t.Fatalf("travel entry %d sequence = %d, want %d", index, expense.Sequence, index)
		}
	}
}
