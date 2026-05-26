package transform

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
	"github.com/willondrik/hoc-spend/pkg/scraper/utils"
)

func Admins(doc *goquery.Document, fiscalYear int, dateRange types.DateRange, fiscalQuarter int) ([]*types.AdminExpenseSummary, error) {

	return []*types.AdminExpenseSummary{}, nil
}

func AdminContractExpenses(doc *goquery.Document) ([]types.AdminContractExpense, error) {
	table := doc.Find(`table.table.contracts.table-striped[summary*="Contracts"]`)
	if table.Length() == 0 {
		return nil, fmt.Errorf("Contracts table not found")
	}

	rows := table.Find("tbody").ChildrenFiltered("tr")
	var entries []types.AdminContractExpense

	for i := 0; i < rows.Length(); i++ {
		row := rows.Eq(i)
		if row.Length() == 0 {
			continue
		}
		cells := row.ChildrenFiltered("td")
		if cells.Length() < 5 {
			continue
		}

		rawSuppliers := strings.TrimSpace(cells.Eq(0).Text())
		description := strings.TrimSpace(cells.Eq(1).Text())
		rawContractPeriod := strings.TrimSpace(cells.Eq(2).Text())
		originalValueRaw := strings.TrimSpace(cells.Eq(3).Text())
		amendedValueRaw := strings.TrimSpace(cells.Eq(4).Text())

		originalValue, err := adminParseMoney(originalValueRaw)
		if err != nil {
			return nil, err
		}
		amendedValue, err := adminParseMoney(amendedValueRaw)
		if err != nil {
			return nil, err
		}

		entries = append(entries, types.AdminContractExpense{
			Suppliers:            adminParseContractSuppliers(rawSuppliers),
			Description:          description,
			ContractPeriod:       adminParseDateRange(rawContractPeriod),
			OriginalContractValue: originalValue,
			AmendedContractValue:  amendedValue,
		})
	}

	return entries, nil
}



// AdministratorExpenditureReports extracts the available quarterly report links
// from the House Administration proactive disclosure page.
//
// This intentionally mirrors MpExpenditureReports so that the orchestration
// layer can treat these report lists consistently.
func AdministratorExpenditureReports(resp *http.Response) (types.ExpenditureReports, error) {
	var expenditureReports types.ExpenditureReports
	var errs []error

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return expenditureReports, fmt.Errorf("[GOQUERY FAILED] %s", err)
	}

	totalReports := 0
	doc.Find("a.quarter").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		totalReports++
		dateRange := s.Find(".quarter-date-range").Text()

		quarter, err := utils.FiscalQuarter(dateRange)
		if err != nil {
			errs = append(errs, fmt.Errorf("Report Quarter error: %s", err))
			return
		}

		reportRange, err := utils.ReportDates(dateRange)
		if err != nil {
			errs = append(errs, fmt.Errorf("Report DateRange error: %s", err))
			return
		}

		year, err := utils.ReportYear(reportRange)
		if err != nil {
			errs = append(errs, fmt.Errorf("Report Year error: %s", err))
			return
		}

		fullHref := href
		if !strings.HasPrefix(href, "http") {
			fullHref = fmt.Sprintf("https://www.ourcommons.ca%s", href)
		}

		expenditureReports.Reports = append(expenditureReports.Reports, types.ExpenditureReport{
			FiscalYear:    year,
			DateRange:     reportRange,
			FiscalQuarter: quarter,
			URL:          fullHref,
		})
	})

	if len(errs) > 0 {
		return expenditureReports, errs[0]
	}

	if totalReports == len(expenditureReports.Reports) {
		log.Printf("All available administrator reports extracted\n\n")
	}
	return expenditureReports, nil
}


// AdminExpenseUrls extracts the three category URLs from a quarterly
// house-administration report page.
//
// It mirrors the script logic: anchors with class "view-report-link".
func AdminExpenseUrls(resp *http.Response) (types.AdministratorExpenseURLs, error) {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return types.AdministratorExpenseURLs{}, fmt.Errorf("[GOQUERY FAILED] %s", err)
	}

	var expenseUrls []string
	doc.Find("a.view-report-link").Each(func(_ int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists {
			return
		}
		url := href
		if !strings.HasPrefix(href, "http") {
			url = fmt.Sprintf("https://www.ourcommons.ca%s", href)
		}
		expenseUrls = append(expenseUrls, url)
	})

	if len(expenseUrls) < 3 {
		return types.AdministratorExpenseURLs{}, fmt.Errorf("expected 3 expense urls, got %d", len(expenseUrls))
	}

	return types.AdministratorExpenseURLs{
		Travel:      expenseUrls[0],
		Hospitality: expenseUrls[1],
		Contract:    expenseUrls[2],
	}, nil
}



func AdminHospitalityExpenses(doc *goquery.Document) ([]types.AdminHospitalityExpense, error) {
	outer := doc.Find(`table.table.expenses.table-striped[summary*="Hospitality Events"]`)
	if outer.Length() == 0 {
		return nil, fmt.Errorf("outer Hospitality Events table not found")
	}

	rows := outer.Find("tbody").ChildrenFiltered("tr")
	var entries []types.AdminHospitalityExpense

	for i := 0; i < rows.Length(); i += 2 {
		mainRow := rows.Eq(i)
		if mainRow.Length() == 0 {
			continue
		}
		detailRow := rows.Eq(i + 1)

		mainCells := mainRow.ChildrenFiltered("td")
		if mainCells.Length() < 5 {
			continue
		}

		eventID := strings.TrimSpace(mainCells.Eq(0).Text())
		host := strings.TrimSpace(mainCells.Eq(1).Text())
		purpose := strings.TrimSpace(mainCells.Eq(2).Text())
		date := strings.TrimSpace(mainCells.Eq(3).Text())
		totalCostRaw := strings.TrimSpace(mainCells.Eq(4).Text())
		totalCost, err := adminParseMoney(totalCostRaw)
		if err != nil {
			return nil, err
		}

		entry := types.AdminHospitalityExpense{
			EventID:   eventID,
			Host:      host,
			Purpose:   purpose,
			Date:      date,
			TotalCost: totalCost,
		}

		if detailRow.Length() > 0 {
			detailRow.Find(`p[aria-labelledby*="Location"]`).Each(func(_ int, p *goquery.Selection) {
				entry.Location = strings.TrimSpace(p.Text())
			})
			detailRow.Find(`p[aria-labelledby*="Suppliers"]`).Each(func(_ int, p *goquery.Selection) {
				supplier := strings.TrimSpace(p.Text())
				entry.Suppliers = adminParseSuppliers(supplier)
			})
			detailRow.Find(`p[aria-labelledby*="TotalAttendees"]`).Each(func(_ int, p *goquery.Selection) {
				entry.Attendance = strings.TrimSpace(p.Text())
			})
		}

		entries = append(entries, entry)
	}

	return entries, nil
}



func AdminTravelExpenses(doc *goquery.Document) ([]types.AdminTravelExpense, error) {
	outer := doc.Find(`table.table.expenses.table-striped[summary*="Travel Events"]`)
	if outer.Length() == 0 {
		return nil, fmt.Errorf("outer Travel Events table not found")
	}

	rows := outer.Find("tbody").ChildrenFiltered("tr")
	var entries []types.AdminTravelExpense

	for i := 0; i < rows.Length(); i += 2 {
		mainRow := rows.Eq(i)
		if mainRow.Length() == 0 {
			continue
		}
		detailRow := rows.Eq(i + 1)

		mainCells := mainRow.ChildrenFiltered("td")
		if mainCells.Length() < 5 {
			continue
		}

		eventID := strings.TrimSpace(mainCells.Eq(0).Text())
		traveller := strings.TrimSpace(mainCells.Eq(1).Text())
		purpose := strings.TrimSpace(mainCells.Eq(2).Text())
		rawDateRange := strings.TrimSpace(mainCells.Eq(3).Text())
		totalCostRaw := strings.TrimSpace(mainCells.Eq(4).Text())
		totalCost, err := adminParseMoney(totalCostRaw)
		if err != nil {
			return nil, err
		}

		var places []string
		var transportation, accommodations, meals, other types.Money

		if detailRow.Length() > 0 {
			detailCell := detailRow.ChildrenFiltered("td").First()

			detailCell.Find(`p[aria-labelledby^="PlacesVisited"]`).Each(func(_ int, p *goquery.Selection) {
				place := strings.TrimSpace(p.Text())
				if place != "" {
					places = append(places, place)
				}
			})

			var parseErr error
			detailCell.Find("table tr").Each(func(_ int, r *goquery.Selection) {
				cols := r.ChildrenFiltered("td")
				if cols.Length() != 2 {
					return
				}
				k := strings.TrimSpace(cols.Eq(0).Text())
				v := strings.TrimSpace(cols.Eq(1).Text())
				val, err := adminParseMoney(v)
				if err != nil {
					parseErr = err
					return
				}

				switch k {
				case "Transportation":
					transportation = val
				case "Accommodations":
					accommodations = val
				case "Meals and Incidentals":
					meals = val
				case "Other":
					other = val
				}
			})
			if parseErr != nil {
				return nil, parseErr
			}
		}

		entries = append(entries, types.AdminTravelExpense{
			EventID:       eventID,
			Traveller:     adminParseName(traveller),
			Purpose:       purpose,
			DateRange:     adminParseDateRange(rawDateRange),
			TotalCost:     totalCost,
			PlacesVisited: places,
			ExpenseBreakdown: types.AdminTravelExpenseBreakdown{
				Transportation:      transportation,
				Accommodations:      accommodations,
				MealsAndIncidentals: meals,
				Other:               other,
			},
		})
	}

	return entries, nil
}




func adminParseDateRange(str string) types.DateRange {
	remove := []string{"\n", "From"}
	for _, v := range remove {
		str = strings.ReplaceAll(str, v, "")
	}

	split := strings.Split(str, "to")
	for i, v := range split {
		split[i] = strings.TrimSpace(v)
	}

	days := []string{
		"Monday, ", "Tuesday, ", "Wednesday, ", "Thursday, ", "Friday, ", "Saturday, ", "Sunday, ",
	}
	for i, v := range split {
		for _, d := range days {
			if strings.Contains(v, d) {
				split[i] = strings.ReplaceAll(v, d, "")
			}
		}
	}

	if len(split) == 1 {
		return types.DateRange{StartDate: split[0], EndDate: split[0]}
	}
	return types.DateRange{StartDate: split[0], EndDate: split[1]}
}

func adminParseName(name string) types.PersonName {
	split := strings.Fields(strings.TrimSpace(name))
	if len(split) == 0 {
		return types.PersonName{}
	}
	if len(split) == 1 {
		return types.PersonName{FirstName: split[0], LastName: ""}
	}
	if len(split) > 2 {
		return types.PersonName{FirstName: split[0], LastName: strings.Join(split[1:], " ")}
	}
	return types.PersonName{FirstName: split[0], LastName: split[1]}
}

func adminParseSuppliers(suppliers string) []string {
	suppliers = strings.TrimSpace(suppliers)
	if suppliers == "" {
		return nil
	}
	if !strings.Contains(suppliers, ",") {
		return []string{suppliers}
	}
	parts := strings.Split(suppliers, ", ")
	return parts
}

func adminFormatToTitleCase(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	split := strings.Split(s, " ")

	for i := 0; i < len(split); i++ {
		if split[i] == "" {
			continue
		}
		r := []rune(split[i])
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		split[i] = string(r)
	}
	return strings.Join(split, " ")
}

func adminParseContractSuppliers(suppliers string) []string {
	suppliers = strings.TrimSpace(suppliers)
	if suppliers == "" {
		return nil
	}

	var s []string
	if strings.Contains(suppliers, "Qualified Supplier(s)") {
		split := strings.Split(suppliers, "•")
		for i := 1; i < len(split); i++ {
			val := adminFormatToTitleCase(split[i])
			val = strings.TrimSpace(val)
			if val != "" {
				s = append(s, val)
			}
		}
		return s
	}

	if strings.Contains(suppliers, "•") {
		split := strings.Split(suppliers, "• ")
		for _, v := range split {
			val := strings.TrimSpace(adminFormatToTitleCase(v))
			if val != "" {
				s = append(s, val)
			}
		}
		return s
	}

	val := adminFormatToTitleCase(suppliers)
	val = strings.TrimSpace(val)
	if val != "" {
		s = append(s, val)
	}
	return s
}

func adminParseMoney(v string) (types.Money, error) {
	return utils.ExpenseToDecimal(strings.TrimSpace(v))
}
