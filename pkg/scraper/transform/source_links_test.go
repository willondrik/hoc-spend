package transform

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestCSVDownloadURL(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<a class="view-report-link csv-btn" href=" /ProactiveDisclosure/DownloadCsv?report=1 ">CSV</a>`))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if got, want := CSVDownloadURL(doc), "https://www.ourcommons.ca/ProactiveDisclosure/DownloadCsv?report=1"; got != want {
		t.Fatalf("CSVDownloadURL() = %q, want %q", got, want)
	}
}

func TestCSVDownloadURLReturnsEmptyWhenAbsent(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`<a class="view-report-link" href="/detail">detail</a>`))
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if got := CSVDownloadURL(doc); got != "" {
		t.Fatalf("CSVDownloadURL() = %q, want empty URL", got)
	}
}
