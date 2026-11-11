package transform

import (
	"reflect"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestMPDirectoryEntries(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(`
<div class="ce-mip-mp-tile-container">
  <a class="ce-mip-mp-tile-link" href="/Members/en/jane-doe(42)"></a>
  <div class="ce-mip-mp-name">Jane Doe</div>
  <div class="ce-mip-mp-constituency">Example Riding</div>
</div>
<div class="ce-mip-mp-tile-container">
  <a class="ce-mip-mp-tile-link" href="/Members/en/missing-name(43)"></a>
  <div class="ce-mip-mp-constituency">Ignored Riding</div>
</div>`))
	if err != nil {
		t.Fatalf("new document: %v", err)
	}

	entries, err := MPDirectoryEntries(doc)
	if err != nil {
		t.Fatalf("MPDirectoryEntries() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	if got, want := entries[0], (types.MPDirectoryEntry{
		Name:         types.PersonName{FirstName: "Jane", LastName: "Doe"},
		Constituency: "Example Riding",
		ProfileURL:   "https://www.ourcommons.ca/Members/en/jane-doe(42)",
	}); !reflect.DeepEqual(got, want) {
		t.Fatalf("MPDirectoryEntries() = %#v, want %#v", got, want)
	}
}

func TestProcessRolesAlwaysIncludesMemberOfParliament(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
	}{
		{
			name: "no additional roles",
			html: `<div class="tab-content"></div>`,
			want: []string{"Member of Parliament"},
		},
		{
			name: "additional roles",
			html: `<div class="tab-content"><div class="ce-mip-roles"><div class="ce-mip-roles-list"><span>Standing Committee on Finance</span><span>Canada-France Inter-Parliamentary Association</span></div></div></div>`,
			want: []string{"Member of Parliament", "Standing Committee on Finance", "Canada-France Inter-Parliamentary Association"},
		},
		{
			name: "designation is not duplicated when supplied by source",
			html: `<div class="tab-content"><div class="ce-mip-roles"><div class="ce-mip-roles-list"><span>Member of Parliament</span><span>Standing Committee on Finance</span></div></div></div>`,
			want: []string{"Member of Parliament", "Standing Committee on Finance"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc, err := goquery.NewDocumentFromReader(strings.NewReader(test.html))
			if err != nil {
				t.Fatalf("create HTML document: %v", err)
			}

			got, err := processRoles(doc.Find(".tab-content"))
			if err != nil {
				t.Fatalf("processRoles returned an error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("processRoles() = %#v; want %#v", got, test.want)
			}
		})
	}
}
