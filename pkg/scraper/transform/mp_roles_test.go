package transform

import (
	"reflect"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

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
