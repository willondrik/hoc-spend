package transform

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestContractSupplierNormalizationAcrossDomains(t *testing.T) {
	t.Run("administration", func(t *testing.T) {
		suppliers := adminParseContractSuppliers("•  aMAZON  • NGP VAN Inc.")
		if got, want := strings.Join(suppliers, "|"), "Amazon|NGP VAN Inc."; got != want {
			t.Fatalf("admin suppliers = %q, want %q", got, want)
		}
	})

	t.Run("officer", func(t *testing.T) {
		doc := supplierDocument(t, `<table><tbody><tr><td>  aMAZON </td><td>Description</td><td>2026-01-01</td><td>$1.00</td></tr></tbody></table>`)
		entries, err := OfficerContractExpenses(doc)
		if err != nil {
			t.Fatalf("OfficerContractExpenses() error = %v", err)
		}
		if len(entries) != 1 || entries[0].Supplier != "Amazon" {
			t.Fatalf("officer entries = %#v, want normalized Amazon", entries)
		}
	})

	t.Run("mp", func(t *testing.T) {
		doc := supplierDocument(t, `<table><tbody><tr><td>  aMAZON </td><td>Description</td><td>2026-01-01</td><td>$1.00</td></tr></tbody></table>`)
		entries, err := MpContractExpenses(doc)
		if err != nil {
			t.Fatalf("MpContractExpenses() error = %v", err)
		}
		if len(entries) != 1 || entries[0].Supplier != "Amazon" {
			t.Fatalf("MP entries = %#v, want normalized Amazon", entries)
		}
	})
}

func supplierDocument(t *testing.T, html string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	return doc
}
