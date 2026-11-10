package utils

import (
	"testing"

	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func TestTravellerNameAndTypeDoesNotCreatePersonForGenericLabel(t *testing.T) {
	travellerType, name := TravellerNameAndType("Designated traveller")
	if travellerType != "Designated Traveller" {
		t.Fatalf("traveller type = %q, want canonical designated traveller label", travellerType)
	}
	if name.FirstName != "" || name.LastName != "" {
		t.Fatalf("generic traveller label produced person name %#v", name)
	}
}

func TestTravellerNameSplitsUnparenthesizedDisplayNamesAndStripsHonorifics(t *testing.T) {
	tests := []struct {
		raw       string
		firstName string
		lastName  string
	}{
		{raw: "Kenny Chiu", firstName: "Kenny", lastName: "Chiu"},
		{raw: "Michela Di Marco", firstName: "Michela", lastName: "Di Marco"},
		{raw: "Right Hon. Jane Doe", firstName: "Jane", lastName: "Doe"},
		{raw: "Doe, Jane", firstName: "Jane", lastName: "Doe"},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			name := TravellerName(test.raw)
			if name.FirstName != test.firstName || name.LastName != test.lastName {
				t.Fatalf("TravellerName(%q) = %#v, want %q %q", test.raw, name, test.firstName, test.lastName)
			}
		})
	}
}

func TestTravellerNameReturnsBlankForNonPersonLabels(t *testing.T) {
	for _, raw := range []string{"", "Not Listed", "Designated Traveller"} {
		t.Run(raw, func(t *testing.T) {
			name := TravellerName(raw)
			if name.FirstName != "" || name.LastName != "" {
				t.Fatalf("TravellerName(%q) = %#v, want blank name", raw, name)
			}
		})
	}
}

func TestNameStripsHonorificFromMPSummary(t *testing.T) {
	var summary types.MPExpenseSummary
	Name("the right hon. Doe, Jane", &summary)
	if summary.Name.FirstName != "Jane" || summary.Name.LastName != "Doe" {
		t.Fatalf("Name() = %#v, want Jane Doe", summary.Name)
	}
}
