package transform

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
	"github.com/willondrik/hoc-spend/pkg/scraper/utils"
)

func OfficerContractExpenses(doc *goquery.Document) ([]types.OfficerContractExpense, error) {
	mainTable := doc.Find("table").First()
	if mainTable.Length() == 0 {
		return nil, fmt.Errorf("table not found")
	}

	rows := mainTable.Find("tbody").First().ChildrenFiltered("tr")
	if rows.Length() == 0 {
		return nil, fmt.Errorf("rows not found")
	}

	var entries []types.OfficerContractExpense
	for i := 0; i < rows.Length(); i++ {
		cells := rows.Eq(i).ChildrenFiltered("td")
		if cells.Length() != 4 {
			continue
		}

		supplier := adminFormatToTitleCase(strings.TrimSpace(cells.Eq(0).Text()))
		if supplier == "" || strings.EqualFold(supplier, "supplier") {
			continue
		}
		desc := strings.TrimSpace(cells.Eq(1).Text())
		date := strings.TrimSpace(cells.Eq(2).Text())
		totalRaw := strings.TrimSpace(cells.Eq(3).Text())

		totalCost, err := utils.ExpenseToDecimal(totalRaw)
		if err != nil {
			return nil, err
		}

		entries = append(entries, types.OfficerContractExpense{
			Supplier:    supplier,
			Description: desc,
			Date:        date,
			TotalCost:   totalCost,
		})
	}

	return entries, nil
}

// OfficerExpenditureReports extracts the available quarterly report links from
// the House Officers proactive disclosure page.
//
// This intentionally mirrors MpExpenditureReports so that the orchestration
// layer can treat these report lists consistently.
func OfficerExpenditureReports(resp *http.Response) (types.ExpenditureReports, error) {
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
			URL:           fullHref,
		})
	})

	if len(errs) > 0 {
		return expenditureReports, errs[0]
	}

	if totalReports == len(expenditureReports.Reports) {
		log.Printf("All available officer reports extracted\n\n")
	}
	return expenditureReports, nil
}

func OfficerHospitalityExpenses(doc *goquery.Document) ([]types.OfficerHospitalityExpense, error) {
	main := doc.Find("table").First()
	if main.Length() == 0 {
		return nil, fmt.Errorf("table not found")
	}

	rows := main.Find("tbody").First().ChildrenFiltered("tr")
	if rows.Length() == 0 {
		return nil, fmt.Errorf("no rows found")
	}

	var entries []types.OfficerHospitalityExpense
	for i := 0; i < rows.Length(); i += 2 {
		outer := rows.Eq(i)
		cells := outer.Find("td")
		if cells.Length() < 5 {
			continue
		}

		date := strings.TrimSpace(cells.Eq(0).Text())
		location := strings.TrimSpace(cells.Eq(1).Text())
		attRaw := strings.TrimSpace(cells.Eq(2).Text())
		purpose := strings.TrimSpace(cells.Eq(3).Text())
		totalRaw := strings.TrimSpace(cells.Eq(4).Text())

		attendance := 0
		if attRaw != "" {
			if v, err := strconv.Atoi(attRaw); err == nil {
				attendance = v
			}
		}

		totalCost, err := utils.ExpenseToDecimal(totalRaw)
		if err != nil {
			return nil, err
		}

		inner := rows.Eq(i + 1)
		iCells := inner.Find("tbody").ChildrenFiltered("tr").Find("td")

		var suppliers []types.OfficerSupplier
		for j := 0; j < iCells.Length(); j += 3 {
			nameRaw := strings.TrimSpace(iCells.Eq(j).Text())
			name := adminFormatToTitleCase(nameRaw)
			costRaw := strings.TrimSpace(iCells.Eq(j + 2).Text())
			if strings.EqualFold(name, "Total") {
				break
			}
			cost, err := utils.ExpenseToDecimal(costRaw)
			if err != nil {
				return nil, err
			}
			if name == "" {
				continue
			}
			suppliers = append(suppliers, types.OfficerSupplier{Name: name, Cost: cost})
		}

		entries = append(entries, types.OfficerHospitalityExpense{
			Date:            date,
			Location:        location,
			TotalAttendance: attendance,
			Purpose:         purpose,
			TotalCost:       totalCost,
			CostPerPerson:   utils.CostPerPerson(totalCost, attendance),
			Suppliers:       suppliers,
		})
	}

	return entries, nil
}

func OfficerTravelExpenses(doc *goquery.Document) ([]types.OfficerTravelExpense, error) {
	// Prefer a table under the common expenses classes, fall back to first table.
	table := doc.Find("table.table")
	if table.Length() == 0 {
		table = doc.Find("table")
	}
	if table.Length() == 0 {
		return nil, fmt.Errorf("table not found")
	}

	rows := table.First().Find("tbody").First().ChildrenFiltered("tr")
	if rows.Length() == 0 {
		return nil, fmt.Errorf("rows not found")
	}

	var entries []types.OfficerTravelExpense
	for i := 0; i < rows.Length(); i += 2 {
		mainRow := rows.Eq(i)
		mainCells := mainRow.ChildrenFiltered("td")
		if mainCells.Length() < 7 {
			continue
		}

		periodRaw := strings.TrimSpace(mainCells.Eq(0).Text())
		period := adminParseDateRange(periodRaw)
		departureCity := strings.TrimSpace(mainCells.Eq(1).Text())
		placesVisitedRaw := strings.TrimSpace(mainCells.Eq(2).Text())

		transportationRaw := strings.TrimSpace(mainCells.Eq(3).Text())
		accommodationRaw := strings.TrimSpace(mainCells.Eq(4).Text())
		mealsRaw := strings.TrimSpace(mainCells.Eq(5).Text())
		totalRaw := strings.TrimSpace(mainCells.Eq(6).Text())

		transportation, err := utils.ExpenseToDecimal(transportationRaw)
		if err != nil {
			return nil, err
		}
		accommodation, err := utils.ExpenseToDecimal(accommodationRaw)
		if err != nil {
			return nil, err
		}
		meals, err := utils.ExpenseToDecimal(mealsRaw)
		if err != nil {
			return nil, err
		}
		total, err := utils.ExpenseToDecimal(totalRaw)
		if err != nil {
			return nil, err
		}

		purpose := ""
		traveller := types.PersonName{}
		if i+1 < rows.Length() {
			detailRow := rows.Eq(i + 1)
			inner := detailRow.Find("table")
			innerRow := inner.Find("tbody").ChildrenFiltered("tr").First()
			if innerRow.Length() > 0 {
				cells := innerRow.ChildrenFiltered("td")
				if cells.Length() >= 2 {
					purpose = strings.TrimSpace(cells.Eq(0).Text())
					rawTraveller := strings.TrimSpace(cells.Eq(1).Text())
					n, err := parseTravellerName(rawTraveller)
					if err != nil {
						return nil, err
					}
					traveller = n
				}
			}
		}

		entries = append(entries, types.OfficerTravelExpense{
			Period:              period,
			DepartureCity:       departureCity,
			PlacesVisited:       parsePlacesVisited(placesVisitedRaw),
			TransportationCost:  transportation,
			AccommodationCost:   accommodation,
			MealsAndIncidentals: meals,
			TotalCost:           total,
			Purpose:             purpose,
			Traveller:           traveller,
		})
	}

	return entries, nil
}

func Officers(doc *goquery.Document, fiscalYear int, dateRange types.DateRange, fiscalQuarter int) ([]*types.OfficerExpenseSummary, error) {
	var officers []*types.OfficerExpenseSummary
	var parseErr error

	rows := doc.Find("tr.expenses-main-info")
	if rows.Length() == 0 {
		return officers, fmt.Errorf("officer rows not found")
	}

	rows.Each(func(_ int, row *goquery.Selection) {
		if parseErr != nil {
			return
		}
		cells := row.Find("td")
		if cells.Length() < 7 {
			return
		}

		rawName := strings.TrimSpace(cells.Eq(0).Text())
		name := parseCommaName(rawName)
		role := strings.TrimSpace(cells.Eq(1).Text())
		caucus := strings.TrimSpace(cells.Eq(2).Text())

		salaryRaw := strings.TrimSpace(cells.Eq(3).Text())
		salary, err := utils.ExpenseToDecimal(salaryRaw)
		if err != nil {
			parseErr = err
			return
		}

		// Keep the text before stripping links; some tables place the amount inside the anchor.
		travelCell := cells.Eq(4)
		travelTotalRaw := strings.TrimSpace(travelCell.Text())
		travelTotal, err := utils.ExpenseToDecimal(travelTotalRaw)
		if err != nil {
			parseErr = err
			return
		}
		travelHref, _ := cells.Eq(4).Find("a.view-report-link").Attr("href")
		if travelHref != "" && !strings.HasPrefix(travelHref, "http") {
			travelHref = fmt.Sprintf("https://www.ourcommons.ca%s", strings.TrimSpace(travelHref))
		}

		// Hospitality: same—text may be inside the link.
		hospCell := cells.Eq(5)
		hospTotalRaw := strings.TrimSpace(hospCell.Text())
		hospTotal, err := utils.ExpenseToDecimal(hospTotalRaw)
		if err != nil {
			parseErr = err
			return
		}
		hospHref, _ := cells.Eq(5).Find("a.view-report-link").Attr("href")
		if hospHref != "" && !strings.HasPrefix(hospHref, "http") {
			hospHref = fmt.Sprintf("https://www.ourcommons.ca%s", strings.TrimSpace(hospHref))
		}

		// Contract: keep text before link removal as well.
		contractCell := cells.Eq(6)
		contractTotalRaw := strings.TrimSpace(contractCell.Text())
		contractTotal, err := utils.ExpenseToDecimal(contractTotalRaw)
		if err != nil {
			parseErr = err
			return
		}
		contractHref, _ := cells.Eq(6).Find("a.view-report-link").Attr("href")
		if contractHref != "" && !strings.HasPrefix(contractHref, "http") {
			contractHref = fmt.Sprintf("https://www.ourcommons.ca%s", strings.TrimSpace(contractHref))
		}

		officers = append(officers, &types.OfficerExpenseSummary{
			Name:                name,
			Role:                role,
			Caucus:              caucus,
			Salaries:            salary,
			TravelExpenses:      types.ExpenseCategory{TotalAmount: travelTotal, URL: travelHref},
			HospitalityExpenses: types.ExpenseCategory{TotalAmount: hospTotal, URL: hospHref},
			ContractExpenses:    types.ExpenseCategory{TotalAmount: contractTotal, URL: contractHref},
			ReportingPeriod:     dateRange,
			FiscalYear:          fiscalYear,
			FiscalQuarter:       fiscalQuarter,
			URL:                 "",
		})
	})

	return officers, parseErr
}
