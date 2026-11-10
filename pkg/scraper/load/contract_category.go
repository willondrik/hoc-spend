package load

import "strings"

const (
	contractCategoryInformationTechnology = "information_technology"
	contractCategoryCommunications        = "communications"
	contractCategoryProfessionalServices  = "professional_services"
	contractCategoryOfficeAndEquipment    = "office_and_equipment"
	contractCategoryFacilities            = "facilities"
	contractCategoryPrintingTranslation   = "printing_and_translation"
	contractCategoryTrainingEvents        = "training_and_events"
	contractCategoryTravel                = "travel"
	contractCategoryOther                 = "other"
)

// contractCategory derives a stable, broad query category from a contract's
// source description. The original description is persisted separately and is
// always retained as the auditable source value.
//
// Rules are deliberately conservative: descriptions that do not clearly match
// a category remain "other" instead of being guessed into a misleading bucket.
func contractCategory(description string) string {
	description = strings.ToLower(strings.TrimSpace(description))

	// More specific rules must precede broader rules. For example, a printing
	// contract can describe promotional communications but remains printing.
	switch {
	case containsAny(description, "print", "printing", "translation", "translate", "interpretation", "interpreter", "transcription"):
		return contractCategoryPrintingTranslation
	case containsAny(description, "software", "licence", "license", "cloud", "hosting", "website", "web site", "web development", "computer", "laptop", "tablet", "cybersecurity", "cyber security", "it support", "information technology"):
		return contractCategoryInformationTechnology
	case containsAny(description, "airfare", "air fare", "flight", "hotel", "accommodation", "transportation", "travel"):
		return contractCategoryTravel
	case containsAny(description, "training", "conference", "seminar", "workshop", "registration", "event"):
		return contractCategoryTrainingEvents
	case containsAny(description, "advertising", "media", "communications", "social media", "newsletter", "photography", "videography", "video production", "graphic design"):
		return contractCategoryCommunications
	case containsAny(description, "consult", "legal", "lawyer", "accounting", "audit", "research", "recruit", "advisory"):
		return contractCategoryProfessionalServices
	case containsAny(description, "rent", "lease", "cleaning", "security", "repair", "maintenance", "utilities", "facility", "facilities"):
		return contractCategoryFacilities
	case containsAny(description, "office supplies", "stationery", "furniture", "equipment", "courier", "postage"):
		return contractCategoryOfficeAndEquipment
	default:
		return contractCategoryOther
	}
}

func containsAny(value string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}
