package transform

import (
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// CSVDownloadURL returns the House of Commons CSV export link on a category
// detail page. The source uses a relative href on production pages.
func CSVDownloadURL(doc *goquery.Document) string {
	if doc == nil {
		return ""
	}

	href, exists := doc.Find("a.csv-btn.view-report-link").First().Attr("href")
	if !exists {
		return ""
	}

	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "http") {
		return href
	}
	return fmt.Sprintf("https://www.ourcommons.ca%s", href)
}
