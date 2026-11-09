package transform

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestOfficersSkipsRowsWithoutAPersonName(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
		<table><tbody>
			<tr class="expenses-main-info"><td>Invalid name</td><td>Role</td><td></td><td>$1.00</td><td>$0.00</td><td>$0.00</td><td>$0.00</td></tr>
			<tr class="expenses-main-info"><td>Doe, Jane</td><td>Role</td><td></td><td>$1.00</td><td>$0.00</td><td>$0.00</td><td>$0.00</td></tr>
		</tbody></table>`))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	officers, err := Officers(doc, 2025, types.DateRange{}, 4)
	if err != nil {
		t.Fatalf("Officers() error = %v", err)
	}
	if len(officers) != 1 || officers[0].Name.FirstName != "Jane" || officers[0].Name.LastName != "Doe" {
		t.Fatalf("Officers() = %#v, want only Jane Doe", officers)
	}
}
