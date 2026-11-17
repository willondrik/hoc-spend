package load

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strings"
)

//go:embed constituency_provinces.csv
var constituencyProvinceCSV string

var constituencyProvinces = mustConstituencyProvinces(constituencyProvinceCSV)

// ProvinceForConstituency resolves the province or territory for a House of
// Commons constituency name. The data is based on the Elections Canada 45th
// general-election district register and includes explicit House source-label
// aliases. It is deliberately independent of MP profile enrichment: report
// rows can represent former members, vacancies, or redistribution-era districts
// that have no current House profile.
func ProvinceForConstituency(constituency string) (string, bool) {
	province, found := constituencyProvinces[constituencyKey(constituency)]
	return province, found
}

func mustConstituencyProvinces(source string) map[string]string {
	rows, err := csv.NewReader(strings.NewReader(source)).ReadAll()
	if err != nil {
		panic(fmt.Sprintf("parse embedded constituency province data: %v", err))
	}

	provinces := make(map[string]string, len(rows))
	for rowIndex, row := range rows {
		if rowIndex == 0 {
			continue
		}
		if len(row) != 2 {
			panic(fmt.Sprintf("parse embedded constituency province data: row %d has %d columns, want 2", rowIndex+1, len(row)))
		}
		key := constituencyKey(row[0])
		province := strings.TrimSpace(row[1])
		if key == "" || province == "" {
			panic(fmt.Sprintf("parse embedded constituency province data: row %d has an empty constituency or province", rowIndex+1))
		}
		if existing, exists := provinces[key]; exists && existing != province {
			panic(fmt.Sprintf("parse embedded constituency province data: conflicting province for %q", row[0]))
		}
		provinces[key] = province
	}
	return provinces
}

func constituencyKey(value string) string {
	replacer := strings.NewReplacer("—", "-", "–", "-", "’", "'", "\u00a0", " ")
	return strings.ToLower(strings.Join(strings.Fields(replacer.Replace(strings.TrimSpace(value))), " "))
}
