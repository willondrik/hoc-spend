package load

import "testing"

func TestContractCategory(t *testing.T) {
	tests := []struct {
		name        string
		description string
		want        string
	}{
		{name: "information technology", description: "Cloud software licence renewal", want: contractCategoryInformationTechnology},
		{name: "case and whitespace normalization", description: "  SOFTWARE LICENSE RENEWAL  ", want: contractCategoryInformationTechnology},
		{name: "communications", description: "Social media video production", want: contractCategoryCommunications},
		{name: "professional services", description: "Legal consulting services", want: contractCategoryProfessionalServices},
		{name: "office and equipment", description: "Office supplies and stationery", want: contractCategoryOfficeAndEquipment},
		{name: "facilities", description: "Constituency office rent and cleaning", want: contractCategoryFacilities},
		{name: "printing and translation", description: "Printing promotional communications materials", want: contractCategoryPrintingTranslation},
		{name: "training and events", description: "Conference registration and training", want: contractCategoryTrainingEvents},
		{name: "travel", description: "Hotel accommodation for travel", want: contractCategoryTravel},
		{name: "blank", description: "", want: contractCategoryOther},
		{name: "unknown", description: "Specialized constituency service", want: contractCategoryOther},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := contractCategory(test.description); got != test.want {
				t.Fatalf("contractCategory(%q) = %q, want %q", test.description, got, test.want)
			}
		})
	}
}
