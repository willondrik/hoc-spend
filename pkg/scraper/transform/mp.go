package transform

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/willondrik/hoc-spend/pkg/scraper/constants"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
	"github.com/willondrik/hoc-spend/pkg/scraper/utils"
)

// MpContractExpenses parses the contract expenses from the MP Contract expenses page
func MpContractExpenses(doc *goquery.Document) ([]*types.ContractExpense, error) {
	var contractExpenses []*types.ContractExpense
	var parseErr error

	doc.Find("tbody tr").Each(func(i int, row *goquery.Selection) {
		contractExpense := &types.ContractExpense{}
		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())

		switch j {
			case 0:
				contractExpense.Supplier = utils.NormalizeSupplierName(text)
			case 1:
				contractExpense.Description = text
			case 2:
				contractExpense.Date = text
			case 3:
			expense, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
				if err != nil {
					parseErr = err
					break
				}
				contractExpense.Total = expense
			contractExpense.IsRefunded = contractExpense.IsRefunded || refunded
			}
		})
		contractExpenses = append(contractExpenses, contractExpense)
	})

	return contractExpenses, parseErr
}



func MpHospitalityExpenses(doc *goquery.Document) ([]*types.HospitalityExpense, error) {
	var hospitalityExpenses []*types.HospitalityExpense
	var parseErr error

	// Debug logging is very verbose; keep it opt-in.
	debug := false
	if v := strings.TrimSpace(os.Getenv("PC_SCRAPER_DEBUG_HOSP")); v != "" && v != "0" {
		debug = true
	}

	rows := doc.Find("tbody tr")
	for i := 0; i < rows.Length(); i++ {
		row := rows.Eq(i)

		if !row.HasClass("expenses-main-info") {
			continue
		}

		if debug {
			fmt.Println("Hospitality expenses outer row: ", row.Length())
			fmt.Println("outer row text:", row.Text())
		}
		

		hospitalityExpense := &types.HospitalityExpense{}
		row.Find("td").Each(func(j int, cell *goquery.Selection) {
			text := strings.TrimSpace(cell.Text())
			switch j {
			case 0:
				hospitalityExpense.Date = text
			case 1:
				hospitalityExpense.Location = text
			case 2:
				text = strings.TrimSpace(text)
				if text == "" {
					hospitalityExpense.Attendance = 0 // Default value for empty attendance
				} else {
					attendance, err := strconv.Atoi(text)
					if err != nil {
						parseErr = err
						return
					}
					hospitalityExpense.Attendance = attendance
				}
			case 3:
				hospitalityExpense.Purpose = text
			case 4:
				cost, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
				if err != nil {
					parseErr = err
					return
				}
				hospitalityExpense.TotalCost = cost
				hospitalityExpense.IsRefunded = hospitalityExpense.IsRefunded || refunded
			}
		})

		// Process hidden data
		hiddenRow := rows.Eq(i + 1)
		if debug {
			fmt.Println("Hospitality inner row length: ", hiddenRow.Length())
			fmt.Println("inner row text:", hiddenRow.Text())
		}
		

		// Retrieve Event type
		eventType := hiddenRow.Find(".col-md-4").Text()
		hospitalityExpense.Event.Type = utils.EventType(eventType)

		var expenseLogs []types.ExpenseItem
		hiddenRow.Find("table tbody tr").Each(func(k int, nestedRow *goquery.Selection) {
			if debug {
				fmt.Println("Hospitality inner row length: ", nestedRow.Length())
				fmt.Println("inner row text:", nestedRow.Text())
			}
		
			var expenseEntry types.ExpenseItem
			nestedRow.Find("td").Each(func(l int, cell *goquery.Selection) {
				text := strings.TrimSpace(cell.Text())
				switch l {
				case 0:
					expenseEntry.Claim = text
				case 1:
					expenseEntry.Supplier = utils.NormalizeSupplierName(text)
			case 2:
				// Costs can be refunds/negatives (e.g. "($40.00)") so parse with refund flag.
				expense, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
				if err != nil {
					parseErr = err
					return
				}
				expenseEntry.Cost = expense
				hospitalityExpense.IsRefunded = hospitalityExpense.IsRefunded || refunded
				}
			})
			if expenseEntry.Claim == "" || expenseEntry.Cost.IsZero() || expenseEntry.Supplier == "" {
				return
			}
			expenseLogs = append(expenseLogs, expenseEntry)
		})
		hospitalityExpense.Event.ExpenseItems = expenseLogs
		hospitalityExpenses = append(hospitalityExpenses, hospitalityExpense)
	}
	//WriteErrs(hospitalityExpenses)
	return hospitalityExpenses, parseErr
}
func WriteErrs(expenses []*types.HospitalityExpense) error {
	if len(expenses) == 0 {
		fmt.Println("No expenses to write.")
		return nil
	}

	var expenseStrings []string
	for _, expense := range expenses {
		expenseStr := fmt.Sprintf("Date: %s, Location: %s, Purpose: %s, Total Cost: %.2f", 
			expense.Date, expense.Location, expense.Purpose, expense.TotalCost)
		expenseStrings = append(expenseStrings, expenseStr)
	}

	content := strings.Join(expenseStrings, "\n") + "\n" // <- Add newline at end
	filePath := "hospitality.txt"

	wd, _ := os.Getwd()
	fmt.Printf("Current working directory: %s\n", wd)
	fmt.Printf("Writing to: %s\n", filePath)

	// 🛠 Open file in append mode
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// 🛠 Append to the file
	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write to %s: %w", filePath, err)
	}

	fmt.Printf("Successfully wrote %d expense(s) to %s\n", len(expenses), filePath)
	return nil
}


func MpExpenditureReports(resp *http.Response) (types.ExpenditureReports, error) {
	var expenditureReports types.ExpenditureReports
	var errs []error

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return expenditureReports, fmt.Errorf("[GOQUERY FAILED] %s", err)
	}


	totalReports := 0
	doc.Find("a.quarter").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists {
			totalReports++
			dateRange := s.Find(".quarter-date-range").Text()

			quarter, err := utils.FiscalQuarter(dateRange)
			if err != nil {
				errs = append(errs, fmt.Errorf("Report Quarter error: %s", err))
				return
			}

			reportRange, err := utils.ReportDates(dateRange)
			if err != nil {
				errs = append(errs,fmt.Errorf("Report DateRange error: %s", err))
				return
			}

			year, err := utils.ReportYear(reportRange)
			if err != nil {
				errs = append(errs, fmt.Errorf("Report Year error: %s", err))
				return
			}

			// Ensure the href has the base URL
			fullHref := href
			if !strings.HasPrefix(href, "http") {
				fullHref = fmt.Sprintf("https://www.ourcommons.ca%s", href)
			}

			expenditureReport := types.ExpenditureReport{
				FiscalYear: year,
				DateRange: reportRange,
				FiscalQuarter: quarter,
				URL: fullHref,
			}

			expenditureReports.Reports = append(expenditureReports.Reports, expenditureReport)
		}
	})

	if len(errs) > 0 {
		return expenditureReports, errs[0]
	}
	
	if totalReports == len(expenditureReports.Reports) {
		log.Printf("All available reports extracted\n\n")
	}
	return expenditureReports, nil
}

func GetPhoneAndFax(s *goquery.Selection) (string, string) {
	linesSplit := strings.Split(strings.TrimSpace(s.Text()), "\n")

	rawPhone := ""
	rawFax := ""

	if len(linesSplit) > 0 {
		rawPhone = strings.ReplaceAll(string(linesSplit[0]), "Telephone:", "")
	}

	if len(linesSplit) > 1 {
		rawFax = strings.ReplaceAll(string(linesSplit[1]), "Fax:", "")
	}

	return strings.TrimSpace(rawPhone), strings.TrimSpace(rawFax)
}
func GetProfile(resp *http.Response, url string) (*types.MPDetails, error) {
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[GOQUERY FAILED] Unable to process resp body: %w", err)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		name     types.PersonName
		roles    []string
		contact  types.MPContactInfo
		province string
		imgData  []byte
		errs     []error
	)

	// Extract name
	name = processName(doc)

	tabCtn := doc.Find(".tab-content")
	profile := doc.Find(".ce-mip-mp-profile-container")

	wg.Add(1)
	go func() {
		defer wg.Done()
		r, err := processRoles(tabCtn)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to process roles: %w", err))
			mu.Unlock()
			return
		}
		roles = r
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		c, err := processContact(tabCtn)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to process contact: %w", err))
			mu.Unlock()
			return
		}
		contact = c
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		p, err := processProvince(profile)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to process province: %w", err))
			mu.Unlock()
			return
		}
		province = strings.TrimSpace(p)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		img, err := processImg(profile)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("failed to process image: %w", err))
			mu.Unlock()
			return
		}
		imgData = img
	}()

	wg.Wait()

	contact.Name = name


	// Check for errors from goroutines
	if len(errs) > 0 {
		// Log errors but continue with default values
		for _, err := range errs {
			fmt.Printf("[WARNING] Error during profile extraction: %v\n", err)
		}
		
		// Set default values for any missing data
		if roles == nil {
			fmt.Println("[WARNING] Setting default roles due to error")
			roles = []string{"Member of Parliament"}
		}
		
		if province == "" {
			fmt.Println("[WARNING] Setting default province due to error")
			province = "Unknown"
		}
		
		if imgData == nil {
			fmt.Println("[WARNING] Setting empty avatar due to error")
			imgData = []byte{}
		}
	}


	fmt.Println("-----------------")
	fmt.Printf("First name: %s\t Last name: %s\n", name.FirstName, name.LastName)
	fmt.Printf("Province: %s\n", province)
	//fmt.Printf("Parliament office: %v\n", contact.HillAddress)
	//fmt.Printf("Constituency office: %v\n", contact.ConstituencyAddress)
	fmt.Printf("Roles: %v\n", roles)
	fmt.Println("-----------------")

	return &types.MPDetails{
		Name:     name,
		Province: province,
		Contact:  contact,
		Roles:    roles,
		Avatar:   imgData,
		ProfileURL: url,
	}, nil
}


func GetUrls(resp *http.Response) ([]string, error) {
	var mpUrls []string

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return mpUrls, fmt.Errorf("failed to parse response body to goquery doc: %s", err)
	}

	doc.Find(".ce-mip-mp-tile-container").Each(func(i int, s *goquery.Selection) {
		rawUrl, exists := s.Find("a").Attr("href")
		if !exists {
			panic("no url")
		}
		url := fmt.Sprintf("https://www.ourcommons.ca%s", rawUrl)
		mpUrls = append(mpUrls, url)
	})
	return mpUrls, nil
}

func IsEmptyFields(v reflect.Value, prefix string) (bool, string) {
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		fieldName := t.Field(i).Name

		if fv.IsZero() {
			return true, fmt.Sprintf("empty field: %s.%s\n", prefix, fieldName)
		}

		if fv.Kind() == reflect.Struct {
			IsEmptyFields(fv, prefix+"."+fieldName)
		}
	}
	return false, ""
}

func getRuntime(elapsed time.Time) {
	fmt.Println(time.Since(elapsed))
}

func processContact(s *goquery.Selection) (types.MPContactInfo, error) {
	var contact types.MPContactInfo
	c := s.Find("#contact")

	// Hill Office	
	hillOffice := types.OfficeAddress{
		Institution: "House of Commons",
		City: "Ottawa",
		Province: "Ontario",
		Country: "Canada",
		PostalCode: "K1A 0A6",
		Description: "Mail may be sent postage-free to any member of Parliament",
	}

	// Extract Email and Website
	// Extract Hill Office
	c.Find("p").Each(func(i int, s *goquery.Selection){

		text := strings.TrimSpace(s.Text())
		switch i {
		case 0:
			contact.Email = text
		case 1:
			contact.Website = text
		case 2:
			return
		case 3:
			phone, fax := GetPhoneAndFax(s)

			hillOffice.PhoneNumber = phone
			hillOffice.FaxNumber = fax
		default: 
			return
		}
	})
	contact.HillOffice = hillOffice

	// Constituency Office
	var constituencyOffice types.OfficeAddress
	var processedOffice bool = false  // Flag to track if we've processed an office
	
	mainOffice := c.Find(".ce-mip-contact-constituency-office")
	mainOffice.Find("p").Each(func(j int, s *goquery.Selection) {
		// Skip if we've already processed an office
		if processedOffice {
		//	fmt.Println("[SKIPPING] Already processed one constituency office.")
			return
		}

		addrSplit := strings.Split(strings.TrimSpace(s.Text()), "\n")
		
		switch j {
		case 0:
			constituencyOffice.Name = addrSplit[0]
			constituencyOffice.StreetAddress = strings.TrimSpace(string(addrSplit[1]))
			constituencyOffice.PostalCode = strings.TrimSpace(addrSplit[3])

			location := strings.TrimSpace(addrSplit[2])
			//fmt.Println("location:", location)
			locationSplit := strings.Split(location, ", ")
			//fmt.Println("split location:", locationSplit)

			// Add debug information
			fmt.Printf("Location: %s\n", location)
			fmt.Printf("Location Split: %+v\n", locationSplit)
			
			// Check if we have enough elements in locationSplit
			if len(locationSplit) > 0 {
				constituencyOffice.City = string(locationSplit[0])
			} else {
				constituencyOffice.City = "Unknown"
				fmt.Println("Warning: Could not extract city from location")
			}
			
			if len(locationSplit) > 1 {
				constituencyOffice.Province = string(locationSplit[1])
			} else {
				constituencyOffice.Province = "Unknown"
				fmt.Println("Warning: Could not extract province from location")
			}
			
			constituencyOffice.Country = "Canada"
		case 1:
			phone, fax := GetPhoneAndFax(s)
			constituencyOffice.PhoneNumber = phone
			constituencyOffice.FaxNumber = fax
			
			// Mark that we've processed one complete office after getting phone/fax
			if constituencyOffice.Name != "" {
				processedOffice = true
				//fmt.Println("[PROCESSED] First constituency office complete.")
			}
		}
	})
	contact.ConstituencyOffice = constituencyOffice


	// Check for empty fields in the Contact struct and its nested structs
	isEmpty, errMsg := IsEmptyFields(reflect.ValueOf(contact), "Contact")
	if isEmpty && errMsg != "" {
		fmt.Printf("Warning: %s - Using default values where possible\n", errMsg)
		
		// Set default values for empty fields
		if contact.Email == "" {
			contact.Email = "not.provided@example.com"
			fmt.Println("Set default email")
		}
		
		if contact.Website == "" {
			contact.Website = "https://www.ourcommons.ca"
			fmt.Println("Set default website")
		}
		
		// Check and set defaults for Hill Office
		if contact.HillOffice.PhoneNumber == "" {
			contact.HillOffice.PhoneNumber = "613-992-0000"
			fmt.Println("Set default Hill Office phone number")
		}
		
		if contact.HillOffice.FaxNumber == "" {
			contact.HillOffice.FaxNumber = "613-992-0000"
			fmt.Println("Set default Hill Office fax")
		}
		
		// Check and set defaults for Constituency Office
		if contact.ConstituencyOffice.Name == "" {
			contact.ConstituencyOffice.Name = "Main Office"
			fmt.Println("Set default Constituency Office name")
		}
		
		if contact.ConstituencyOffice.StreetAddress == "" {
			contact.ConstituencyOffice.StreetAddress = "Unknown"
			fmt.Println("Set default Constituency Office street address")
		}
		
		if contact.ConstituencyOffice.City == "" {
			contact.ConstituencyOffice.City = "Unknown"
			fmt.Println("Set default Constituency Office city")
		}
		
		if contact.ConstituencyOffice.Province == "" {
			contact.ConstituencyOffice.Province = "Unknown"
			fmt.Println("Set default Constituency Office province")
		}
		
		if contact.ConstituencyOffice.PhoneNumber == "" {
			contact.ConstituencyOffice.PhoneNumber = "000-000-0000"
			fmt.Println("Set default Constituency Office phone number")
		}
		
		if contact.ConstituencyOffice.FaxNumber == "" {
			contact.ConstituencyOffice.FaxNumber = "000-000-0000"
			fmt.Println("Set default Constituency Office fax")
		}
	}

	return contact, nil
}


func processImg(s *goquery.Selection) ([]byte, error) {
	fmt.Println("[PROCESSING IMG]...")
	imgSrc, exists := s.Find(".ce-mip-mp-picture").Attr("src")
	if !exists {
		fmt.Println("[WARNING] Image source doesn't exist, using empty avatar")
		return []byte{}, nil
	}

	url := fmt.Sprintf("https://www.ourcommons.ca%s", imgSrc)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[WARNING] GET request failed for image: %v, using empty avatar\n", err)
		return []byte{}, nil
	}
	defer resp.Body.Close()

	imgData, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("[WARNING] Failed to extract image data: %v, using empty avatar\n", err)
		return []byte{}, nil
	}

	fmt.Println("[SUCCESS] Image extracted.")
	return imgData, nil
}

func processName(doc *goquery.Document) types.PersonName {
	//fmt.Println("[PROCESSING NAME]...")
	nameHdr := doc.Find("h1")
	name := strings.TrimSpace(nameHdr.Text())
	formattedName := formatName(name)
	//fmt.Println("[SUCCESS] Name extracted:", "First name: ", formattedName.FirstName, "Last name: ", formattedName.LastName)
	return formattedName
}

func formatName(name string) types.PersonName {
	
	
	name = removeNamePrefix(name)

	// Find the first space character
	spaceIndex := strings.Index(name, " ")
	if spaceIndex > 0 {
		firstName := name[:spaceIndex]
		lastName := name[spaceIndex+1:]
		return types.PersonName{
			FirstName: firstName,
			LastName: lastName,
		}
	}
	
	// Fallback to Fields for other cases
	splitName := strings.Fields(name)
	
	firstName := splitName[0]
	
	// Join all remaining elements as the last name (handles multi-part last names)
	lastName := strings.Join(splitName[1:], " ")
	
	return types.PersonName{
		FirstName: firstName,
		LastName: lastName,
	}
}

func removeNamePrefix(name string) string {
	if strings.Contains(name, "The Honourable ") {
		return strings.ReplaceAll(name, "The Honourable ", "")
	}

	if strings.Contains(name, "The Right Honourable ") {
		return strings.ReplaceAll(name, "The Right Honourable ", "")
	}

	return name
}

func processProvince(s *goquery.Selection) (string, error) {
	fmt.Println("[PROCESSING PROVINCE]...")
	var province string
	s.Find("dd").Each(func(i int, s *goquery.Selection) {
		if i == 2 {
			province = strings.TrimSpace(s.Text())
		}
	})

	if province == "" {
		fmt.Println("[WARNING] Province field is empty, using 'Unknown'")
		return "Unknown", nil
	}

	fmt.Println("[SUCCESS] Province extracted:", province)
	return province, nil
}

func processRoles(s *goquery.Selection) ([]string, error) {
	fmt.Println("[PROCESSING ROLES]...")
	var roles []string
	r := s.Find("#roles")
	list := r.Find(".ce-mip-roles-list")

	list.Find("span").Each(func(i int, s *goquery.Selection) {
		roles = append(roles, strings.TrimSpace(s.Text()))
	})
	fmt.Println("[SUCCESS] Roles extracted:", roles)

	if len(roles) < 1 {
		fmt.Println("[WARNING] No roles found for MP, using default role 'Member of Parliament'")
		return []string{"Member of Parliament"}, nil
	}

	return roles, nil
}



func Mps(doc *goquery.Document, fiscalYear int, dateRange types.DateRange, fiscalQuarter int) ([]*types.MPExpenseSummary, error) {
	var mps []*types.MPExpenseSummary
	var parseErr error

	dataTable := doc.Find("#data-table")
	dataTable.Find("tbody tr").Each(func(i int, row *goquery.Selection) {

		mp := &types.MPExpenseSummary{}
		row.Find("td").Each(func(j int, cell *goquery.Selection) {

			text := strings.TrimSpace(cell.Text())
			switch j {
			case 0:
				utils.Name(text, mp)
			case 1:
				if len(text) < 1 {
					fmt.Println("NO TEXT FOR CONSTITUENCY", mp.Name)
					panic("no text for constituency")
				}
				if text == "" {
					mp.Constituency = "None"
				} else {
					mp.Constituency = text
				}
			case 2:
				mp.Caucus = text
			case 3:
				salaries, err := utils.ExpenseToDecimal(text)
				if err != nil {
					parseErr = err
					return
				}
				mp.Salaries = salaries
			case 4:
				travelExpenses, err := utils.ExpenseToDecimal(text)
				if err != nil {
					parseErr = err
					return
				}
				mp.TravelExpenses.TotalAmount = travelExpenses

				href, err := utils.GetHref(cell)
				if err != nil {
					parseErr = err
					return
				}
				mp.TravelExpenses.URL = href
			case 5:
				hospitalityExpenses, err := utils.ExpenseToDecimal(text)
				if err != nil {
					parseErr = err
					return
				}
				mp.HospitalityExpenses.TotalAmount = hospitalityExpenses

				href, err := utils.GetHref(cell)
				if err != nil {
					parseErr = err
					return
				}
				mp.HospitalityExpenses.URL = href
			case 6:
				contractExpenses, err := utils.ExpenseToDecimal(text)
				if err != nil {
					parseErr = err
					return
				}
				mp.ContractExpenses.TotalAmount = contractExpenses

				href, err := utils.GetHref(cell)
				if err != nil {
					parseErr = err
				}
				mp.ContractExpenses.URL = href
			default:
				fmt.Println("Unexpected column")

			}
		})

		if parseErr == nil {
			mp.Name = utils.UpdateVacantNames(mp.Name, mp.Constituency)
			// Set fiscal year and date information
			mp.FiscalYear = fiscalYear
			mp.ReportingPeriod = dateRange
			mp.FiscalQuarter = fiscalQuarter
			mps = append(mps, mp)
		} else {
			log.Printf("\n\nError parsing %s %s: %+v", mp.Name.FirstName, mp.Name.LastName, parseErr)
		}
	})

	return mps, parseErr
}



func MpTravelExpenses(doc *goquery.Document) ([]*types.TravelExpense, error) {
	// Debug logging is extremely verbose; keep it opt-in.
	debug := false
	if v := strings.TrimSpace(os.Getenv("PC_SCRAPER_DEBUG_TRAVEL")); v != "" && v != "0" {
		debug = true
	}

	if debug {
		fmt.Println("Extracting travel expenses...")
	}
	var mpTravelExpenses []*types.TravelExpense
	var parseErr error
	seq := 0

	rows := doc.Find("tbody tr")
	for i := 0; i < rows.Length(); i++ {
		row := rows.Eq(i)

		if row.HasClass("expenses-main-info") {
			cells := row.Find("td")
			if debug {
				fmt.Printf("Found expenses-main-info row with %d cells\n", cells.Length())
				cells.Each(func(i int, cell *goquery.Selection) {
					text := strings.TrimSpace(cell.Text())
					fmt.Printf("Cell %d: '%s'\n", i, text)
				})
			}
			

			// Skip invalid rows
			if cells.Length() == 0 || strings.Contains(strings.ToLower(cells.Text()), "traveller") || strings.TrimSpace(cells.Eq(0).Text()) == "" {
				if debug {
					fmt.Println("Skipping invalid or header row")
				}
				continue
			}

			// CASE 1: 8-column row
			if cells.Length() == 8 {
				travelExpense := &types.TravelExpense{Sequence: seq}
				travelExpense.Claim = "Not Provided"
				var departureCity, destinationCity string

				cells.Each(func(j int, cell *goquery.Selection) {
					text := strings.TrimSpace(cell.Text())
					switch j {
					case 0:
						dates, err := utils.StringToDateRange(text)
						if err != nil {
							parseErr = err
							return
						}
						travelExpense.DateRange = dates
					case 1:
						departureCity = utils.CityName(text)
					case 2:
						destinationCity = utils.CityName(text)
					case 3:
						cost, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
						if err != nil {
							parseErr = err
							return
						}
						travelExpense.Costs.Transportation = cost
						travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					case 4:
						cost, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
						if err != nil {
							parseErr = err
							return
						}
						travelExpense.Costs.Accommodation = cost
						travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					case 5:
						cost, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
						if err != nil {
							parseErr = err
							return
						}
						travelExpense.Costs.MealsAndIncidentals = cost
						travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					case 6:
						cost, refunded, err := utils.ExpenseToDecimalWithRefundFlag(text)
						if err != nil {
							parseErr = err
							return
						}
						travelExpense.Costs.Total = cost
						travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					}
				})

				// Hidden nested table
				if i+1 < rows.Length() {
					hiddenRow := rows.Eq(i + 1)
					if hiddenRow.Find("table").Length() > 0 {
						hiddenRow.Find("table tbody tr").Each(func(k int, nestedRow *goquery.Selection) {
							if k == 0 || strings.TrimSpace(nestedRow.Text()) == "" {
								return
							}
							var traveller types.Traveller
							var purpose, travelerInfo string
							nestedRow.Find("td").Each(func(l int, cell *goquery.Selection) {
								text := strings.TrimSpace(cell.Text())
								if l == 0 {
									purpose = text
								} else if l == 1 {
									travelerInfo = text
								}
							})
							traveller.Purpose = purpose
							traveller.Type, traveller.Name = utils.TravellerNameAndType(travelerInfo)

							// Set date
							if _, err := time.Parse("2006-01-02", travelExpense.DateRange.StartDate); err == nil {
								traveller.Date = travelExpense.DateRange.StartDate
							} else {
								traveller.Date = "1970-01-01"
							}

							traveller.DepartureCity = utils.CityName(departureCity)
							traveller.DestinationCity = utils.CityName(destinationCity)


							
							if utils.IsFlight(travelExpense.FlightPoints) {
								traveller.TransportationMode = constants.AIR_TRANSPORTATION
							} else {
								traveller.TransportationMode = constants.GROUND_TRANSPORTATION
							}

							travelExpense.Travellers = append(travelExpense.Travellers, traveller)
						})
						i++
					}
				}
				if debug {
					fmt.Printf("travel expense: %+v\n\n", travelExpense)
				}
				mpTravelExpenses = append(mpTravelExpenses, travelExpense)
				seq++

			} else {
				// CASE 2: Full row
			travelExpense := &types.TravelExpense{Sequence: seq}
				row.Find("td").Each(func(j int, cell *goquery.Selection) {
					text := strings.TrimSpace(cell.Text())
					switch j {
					case 0:
						travelExpense.Claim = text
					case 1:
						dates, err := utils.StringToDateRange(text)
						if err == nil {
							travelExpense.DateRange = dates
						}
					case 2:
					val, refunded, _ := utils.ExpenseToDecimalWithRefundFlag(text)
					travelExpense.Costs.Transportation = val
					travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					case 3:
					val, refunded, _ := utils.ExpenseToDecimalWithRefundFlag(text)
					travelExpense.Costs.Accommodation = val
					travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					case 4:
					val, refunded, _ := utils.ExpenseToDecimalWithRefundFlag(text)
					travelExpense.Costs.MealsAndIncidentals = val
					travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					case 5:
						travelExpense.FlightPoints.Regular, _ = utils.FlightPointsToFloat(text)
					case 6:
						travelExpense.FlightPoints.Special, _ = utils.FlightPointsToFloat(text)
					case 7:
						travelExpense.FlightPoints.USA, _ = utils.FlightPointsToFloat(text)
					case 8:
					val, refunded, _ := utils.ExpenseToDecimalWithRefundFlag(text)
					travelExpense.Costs.Total = val
					travelExpense.IsRefunded = travelExpense.IsRefunded || refunded
					}
				})

				// Check if nested row with travellers exists
				if i+1 < rows.Length() {
					hiddenRow := rows.Eq(i + 1)
					if hiddenRow.Find("table").Length() > 0 {
						hiddenRow.Find("table tbody tr").Each(func(k int, nestedRow *goquery.Selection) {
							if strings.TrimSpace(nestedRow.Text()) == "" {
								return
							}

							var traveller types.Traveller
							numCols := nestedRow.Find("td").Length()

							nestedRow.Find("td").Each(func(l int, cell *goquery.Selection) {
								text := strings.TrimSpace(cell.Text())
								switch numCols {
								case 4:
									switch l {
									case 0:
										traveller.Name = utils.TravellerName(text)
									case 1:
										traveller.Type = text
									case 2:
										traveller.Purpose = text
									case 3:
										city := utils.City(text)
										traveller.DepartureCity = utils.CityName(city)
										traveller.DestinationCity = utils.CityName(city)
									}
									if utils.IsFlight(travelExpense.FlightPoints) {
										traveller.TransportationMode = constants.AIR_TRANSPORTATION
									} else {
										traveller.TransportationMode = constants.GROUND_TRANSPORTATION
									}
									traveller.Date = travelExpense.DateRange.StartDate
								default:
									switch l {
									case 0:
										traveller.Name = utils.TravellerName(text)
									case 1:
										traveller.Type = text
									case 2:
										traveller.Purpose = text
									case 3:
										traveller.Date = text
									case 4:
										traveller.DepartureCity = utils.CityName(text)
									case 5:
										traveller.DestinationCity = utils.CityName(text)
									}
									if utils.IsFlight(travelExpense.FlightPoints) {
										traveller.TransportationMode = constants.AIR_TRANSPORTATION
									} else {
										traveller.TransportationMode = constants.GROUND_TRANSPORTATION
									}								
								}
							})

							if traveller.Name.FirstName == "" || traveller.Type == "" {
								return
							}

							if traveller.Date == "" {
								
								isCancelledTrip := utils.IsTripCancelled(traveller.Purpose)
								if isCancelledTrip {
									utils.HandleCancelledTrip(&traveller, travelExpense.DateRange)
								}

								isLoungeVisit := utils.IsLoungeVisit(traveller.Purpose)
								if isLoungeVisit {
									utils.HandleLoungeVisit(&traveller, travelExpense.DateRange)
								}
							}

							travelExpense.Travellers = append(travelExpense.Travellers, traveller)
						})
						i++
					}
				}
				mpTravelExpenses = append(mpTravelExpenses, travelExpense)
				seq++
			}
		}
	}

	return mpTravelExpenses, parseErr
}
