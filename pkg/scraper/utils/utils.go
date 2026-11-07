package utils

import (
	"bufio"
	"fmt"
	"os"

	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/shopspring/decimal"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

const baseURL = "https://www.ourcommons.ca"

func ReverseDirectionKey(cities string) string {
	split := strings.Split(cities, "_")

	return fmt.Sprintf("%s_%s", split[1], split[0])
}

// GetHref extracts the href attribute from a cell in a table
// and constructs the full URL using the base URL
func GetHref(cell *goquery.Selection) (string, error) {
	link := cell.Find("a")
	hrefText, _ := link.Attr("href")
	hrefText = strings.TrimSpace(hrefText)
	if hrefText == "" {
		return "", nil
	}

	href := fmt.Sprintf("%s%s", baseURL, hrefText)
	return href, nil
}

// GetText extracts the text from a cell in a qoquery table
func FiscalQuarter(text string) (int, error) {
	var quarter int
	var parseErr error
	switch {
	case strings.Contains(text, "First"):
		quarter = 1
	case strings.Contains(text, "Second"):
		quarter = 2
	case strings.Contains(text, "Third"):
		quarter = 3
	case strings.Contains(text, "Fourth"):
		quarter = 4
	default:
		quarter = -1
		parseErr = fmt.Errorf("Failed to extract quarter. Unknown entry")
	}
	return quarter, parseErr
}

// ReportDates extracts the date range from a string
// and converts it to a DateRange struct
func ReportDates(text string) (types.DateRange, error) {

	// Split by newlines and get the first line
	splitText := strings.Split(text, "\n")
	if len(splitText) == 0 {
		return types.DateRange{}, fmt.Errorf("invalid text format: no lines found")
	}

	firstLine := strings.TrimSpace(splitText[0])

	// Try different dash separators
	var parts []string
	for _, separator := range []string{" – ", " - "} {
		parts = strings.Split(firstLine, separator)
		if len(parts) >= 2 {
			break
		}
	}

	if len(parts) < 2 {
		// Try to find "From" and "to" pattern
		if strings.Contains(firstLine, "From") && strings.Contains(firstLine, "to") {
			fromIndex := strings.Index(firstLine, "From")
			dateText := firstLine[fromIndex:]
			dateText = strings.ReplaceAll(dateText, "From ", "")

			dates := strings.Split(dateText, " to ")
			if len(dates) >= 2 {
				startDate := strings.TrimSpace(dates[0])
				endDate := strings.TrimSpace(dates[1])

				// Check if already in Postgres format
				if IsPostgresDate(startDate) && IsPostgresDate(endDate) {
					return types.DateRange{
						StartDate: startDate,
						EndDate:   endDate,
					}, nil
				}

				dateRange := types.DateRange{
					StartDate: startDate,
					EndDate:   endDate,
				}

				formattedDateRange, err := ConvertDateFormat(dateRange)
				if err != nil {
					// If conversion fails, return original dates
					return dateRange, nil
				}

				return formattedDateRange, nil
			}
		}

		return types.DateRange{}, fmt.Errorf("invalid text format: missing date range separator")
	}

	// Extract the date range part
	dateRangeStr := strings.TrimSpace(parts[1])
	dateRangeStr = strings.ReplaceAll(dateRangeStr, "From ", "")

	// Split by "to" to get start and end dates
	dates := strings.Split(dateRangeStr, " to ")
	if len(dates) < 2 {
		return types.DateRange{}, fmt.Errorf("invalid date range format: missing 'to' separator")
	}

	startDate := strings.TrimSpace(dates[0])
	endDate := strings.TrimSpace(dates[1])

	// Check if already in Postgres format
	if IsPostgresDate(startDate) && IsPostgresDate(endDate) {
		return types.DateRange{
			StartDate: startDate,
			EndDate:   endDate,
		}, nil
	}

	dateRange := types.DateRange{
		StartDate: startDate,
		EndDate:   endDate,
	}

	formattedDateRange, err := ConvertDateFormat(dateRange)
	if err != nil {
		// If conversion fails, return the original dates
		return dateRange, nil
	}

	return formattedDateRange, nil
}

// ReportYear extracts the year from a date range
func ReportYear(dateRange types.DateRange) (int, error) {

	// Handle empty date
	if dateRange.StartDate == "" {
		// Default to current year if date is empty
		currentYear := time.Now().Year()
		return currentYear, nil
	}

	// Check if the date is in Postgres format
	if IsPostgresDate(dateRange.StartDate) {
		startDateSplit := strings.Split(dateRange.StartDate, "-")

		if len(startDateSplit) >= 1 {
			// Handle empty year part
			yearStr := strings.TrimSpace(startDateSplit[0])

			if yearStr != "" {
				year, err := strconv.Atoi(yearStr)
				if err == nil {
					return year, nil
				}
			}
		}
	}

	// If year cannot be extracted, try to parse it as a full date
	for _, format := range []string{
		"2006-01-02",
		"January 2, 2006",
		"Jan 2, 2006",
		"2006/01/02",
	} {
		t, err := time.Parse(format, dateRange.StartDate)
		if err == nil {
			year := t.Year()
			return year, nil
		}
	}

	// If all fails, use the current year
	currentYear := time.Now().Year()
	return currentYear, nil
}

// IsWaterCrossing checks if the trip between two cities involves crossing a body of water
// This is used to identify trips that would be flights despite being short distances
func IsWaterCrossing(departureCity, destinationCity string) bool {
	// Define pairs of cities that involve water crossings
	waterCrossingPairs := map[string]bool{
		// Vancouver to Victoria (across the Strait of Georgia)
		"Vancouver_Victoria": true,
		"Victoria_Vancouver": true,

		// Charlottetown (PEI) to Halifax (NS) - across water
		"Charlottetown_Halifax": true,
		"Halifax_Charlottetown": true,

		// Sydney (NS) to St. John's (NL) - across the Cabot Strait
		"Sydney_St. John's": true,
		"St. John's_Sydney": true,

		// Moncton (NB) to Charlottetown (PEI) - short distance but across water
		"Moncton_Charlottetown": true,
		"Charlottetown_Moncton": true,
	}

	// Create a key for the city pair
	cityPair := fmt.Sprintf("%s_%s", departureCity, destinationCity)

	// Check if this pair is in our water crossing map
	return waterCrossingPairs[cityPair]
}

// IsFlight checks if the travel purpose indicates a flight based on flight points
// This is the original function kept for backward compatibility
func IsFlight(flightPoints types.FlightPoints) bool {
	if flightPoints.Regular != 0 || flightPoints.Special != 0 || flightPoints.USA != 0 {
		return true
	}
	return false
}

// IsFlightWithDistance checks if the travel purpose indicates a flight
// and if the departure and destination cities are valid flight pairs
// Uses flight points, distance between cities, and water crossings to determine if it's a flight
func IsFlightWithDistance(flightPoints types.FlightPoints, departureCity, destinationCity string, distanceKm float64) bool {
	// If there are flight points, it's likely a flight
	if flightPoints.Regular != 0 || flightPoints.Special != 0 || flightPoints.USA != 0 {
		// But check if it's a short distance and not a water crossing
		// Some entries might have flight points but are actually drive trips
		if distanceKm > 0 && distanceKm < 300 && !IsWaterCrossing(departureCity, destinationCity) {
			return false
		}
		return true
	}

	// If no flight points but it's a long distance or a water crossing, it's likely a flight
	if distanceKm >= 300 || IsWaterCrossing(departureCity, destinationCity) {
		return true
	}

	return false
}

// WriteFlightErrorsToFile writes flight error messages to a file
// Aids in debugging and tracking issues
// Can check the file for specific cases that are failing
func WriteFlightErrorsToFile(errorMsgs []string) error {
	if len(errorMsgs) == 0 {
		fmt.Println("No errors to write.")
		return nil
	}

	content := strings.Join(errorMsgs, "\n") + "\n" // Add newline at the end
	filePath := "sample.txt"

	// Confirm where we are writing from
	wd, _ := os.Getwd()
	fmt.Printf("Current working directory: %s\n", wd)
	fmt.Printf("Writing to: %s\n", filePath)

	// Open file in append mode, create if it doesn't exist
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Write content to file
	if _, err := file.WriteString(content + "\n"); err != nil {
		return fmt.Errorf("failed to write to %s: %w", filePath, err)
	}

	fmt.Printf("Successfully wrote %d error(s) to %s\n", len(errorMsgs), filePath)
	return nil
}

// UpdateVacantNames updates the names of MPs with "Vacant" in their name
// Unique constraints in the database prevent duplicates,
// and this function ensures that the name is unique by appending the constituency
func UpdateVacantNames(name types.PersonName, constituency string) types.PersonName {
	if name.FirstName == "Vacant" || name.LastName == "Vacant" {
		return types.PersonName{
			FirstName: fmt.Sprintf("%s - %s", name.FirstName, constituency),
			LastName:  fmt.Sprintf("%s - %s", name.LastName, constituency),
		}
	}
	return name
}

func Name(fullName string, mp *types.MPExpenseSummary) {
	var firstName string
	var lastName string

	switch fullName {

	case "Vacant":
		firstName = "Vacant"
		lastName = "Vacant"

	default:
		var formattedStr string

		if strings.Contains(fullName, "Right Hon. ") {
			formattedStr = strings.ReplaceAll(fullName, "Right Hon. ", "")
		} else if strings.Contains(fullName, "Hon. ") {
			formattedStr = strings.ReplaceAll(fullName, "Hon. ", "")
		} else {
			formattedStr = fullName
		}

		// Check if the name contains a comma before splitting
		if strings.Contains(formattedStr, ", ") {
			names := strings.Split(formattedStr, ", ")
			firstName = strings.TrimSpace(names[1])
			lastName = strings.TrimSpace(names[0])
		} else {
			// Handle case where name doesn't have a comma
			// Set the entire name as LastName
			firstName = ""
			lastName = formattedStr
		}
	}
	mp.Name = types.PersonName{
		FirstName: firstName,
		LastName:  lastName,
	}
}

// TravellerNameAndType extracts the traveller type and name from a string
// TODO: Refactor - some of this is unnecessary
func TravellerNameAndType(travellerDetails string) (string, types.PersonName) {

	// Check if it contains a name in parentheses
	if strings.Contains(travellerDetails, "(") && strings.Contains(travellerDetails, ")") {
		split := strings.SplitN(travellerDetails, "(", 2)
		travellerType := strings.TrimSpace(split[0])

		// Clean and extract name
		nameRaw := strings.TrimSpace(strings.TrimRight(split[1], ")"))
		nameSlice := strings.Split(nameRaw, ", ")

		if len(nameSlice) == 2 {
			return travellerType, types.PersonName{
				FirstName: strings.TrimSpace(nameSlice[1]),
				LastName:  strings.TrimSpace(nameSlice[0]),
			}
		}

		// Fallback if name doesn't split as expected
		return travellerType, types.PersonName{
			FirstName: "",
			LastName:  nameRaw,
		}
	}

	// No parentheses - treat whole string as name or type
	return "Unknown", types.PersonName{
		FirstName: "",
		LastName:  strings.TrimSpace(travellerDetails),
	}
}

// ExpenseToDecimal converts a string representation of an expense to a Money value.
// It preserves existing behaviour and is a thin wrapper over ExpenseToDecimalWithRefundFlag.
func ExpenseToDecimal(expenseTotal string) (types.Money, error) {
	amount, _, err := ExpenseToDecimalWithRefundFlag(expenseTotal)
	return amount, err
}

// CostPerPerson returns a hospitality event's total cost divided by its
// attendance, rounded to cents. A non-positive attendance has no meaningful
// per-person cost, so it returns zero.
func CostPerPerson(totalCost types.Money, attendance int) types.Money {
	if attendance <= 0 {
		return decimal.Zero
	}

	return totalCost.Div(decimal.NewFromInt(int64(attendance))).Round(2)
}

// ExpenseToDecimalWithRefundFlag parses currency strings and also returns a boolean
// indicating if the value was parenthesis-negative (treated as refunded). Parentheses
// are commonly used on the site to denote refunds.
func ExpenseToDecimalWithRefundFlag(expenseTotal string) (types.Money, bool, error) {
	// Handle empty strings
	expenseTotal = strings.TrimSpace(expenseTotal)
	if expenseTotal == "" || expenseTotal == "-" {
		return decimal.Zero, false, nil
	}

	trimmedExpense := strings.TrimSpace(expenseTotal)

	// Parentheses are sometimes used to denote negative values.
	// e.g. "($1,234.56)".
	isParenNegative := strings.HasPrefix(trimmedExpense, "(") && strings.HasSuffix(trimmedExpense, ")")
	trimmedExpense = strings.Trim(trimmedExpense, "()")

	// Some pages render negative currency as "-$123.45".
	// Normalize to "-123.45".
	if strings.HasPrefix(trimmedExpense, "-$") {
		trimmedExpense = "-" + strings.TrimPrefix(trimmedExpense, "-$")
	} else {
		trimmedExpense = strings.TrimPrefix(trimmedExpense, "$")
	}

	trimmedExpense = strings.ReplaceAll(trimmedExpense, ",", "")
	trimmedExpense = strings.TrimSpace(trimmedExpense)
	if trimmedExpense == "" {
		return decimal.Zero, isParenNegative, nil
	}

	expenseDecimal, err := decimal.NewFromString(trimmedExpense)
	if err != nil {
		return decimal.Zero, false, fmt.Errorf("Failed to parse string: '%s'\nError: %v", trimmedExpense, err)
	}

	if isParenNegative {
		expenseDecimal = expenseDecimal.Neg()
	}

	// Quantize to 2 decimal places (cents) to ensure consistent downstream
	// float64 conversion and aggregation behaviour.
	return expenseDecimal.Round(2), isParenNegative, nil
}

// StringToDateRange converts a string representation of a date range into a DateRange struct
func StringToDateRange(dateStr string) (types.DateRange, error) {
	dateStr = strings.TrimSpace(dateStr)

	if dateStr == "" {
		return types.DateRange{
			StartDate: "2000-02-02",
			EndDate:   "2000-02-02",
		}, nil
	}

	// Normalize spacing and remove optional "From"
	dateStr = strings.ReplaceAll(dateStr, "From ", "")
	dateStr = strings.ReplaceAll(dateStr, "from ", "")
	dateStr = strings.Join(strings.Fields(dateStr), " ") // Collapse spaces

	// Safe "to" range check
	if strings.Contains(dateStr, " to ") {
		dateArr := strings.Split(dateStr, " to ")
		if len(dateArr) == 2 {
			startDate := strings.TrimSpace(dateArr[0])
			endDate := strings.TrimSpace(dateArr[1])

			// Check if already in PSQL format
			if IsPostgresDate(startDate) && IsPostgresDate(endDate) {
				return types.DateRange{StartDate: startDate, EndDate: endDate}, nil
			}

			// Try conversion
			dateRange := types.DateRange{StartDate: startDate, EndDate: endDate}
			formatted, err := ConvertDateFormat(dateRange)
			if err != nil {
				return dateRange, nil
			}
			return formatted, nil
		}
	}

	// Handle single PSQL-style date
	if IsPostgresDate(dateStr) {
		return types.DateRange{
			StartDate: dateStr,
			EndDate:   dateStr,
		}, nil
	}

	// Handle malformed but dash-containing ranges
	if strings.Contains(dateStr, "-") {
		parts := strings.Fields(dateStr)
		if len(parts) == 2 {
			return types.DateRange{
				StartDate: parts[0],
				EndDate:   parts[1],
			}, nil
		}
	}

	// Final fallback
	return types.DateRange{
		StartDate: dateStr,
		EndDate:   dateStr,
	}, nil
}

// FlightPointsToFloat converts a string representation of flight points to a float64
func FlightPointsToFloat(flightPoints string) (float64, error) {

	// Handle empty strings
	flightPoints = strings.TrimSpace(flightPoints)
	if flightPoints == "" {
		return 0, nil
	}

	pointsFloat, err := strconv.ParseFloat(flightPoints, 64)
	if err != nil {
		return -1, fmt.Errorf("Failed to parse string: %s\nError: %v", flightPoints, err)
	}

	return pointsFloat, nil
}

// TravellerName extracts the name of the traveller from a string
// It handles cases where the name is in parentheses or contains "Right Hon." or "Hon."
// It also handles cases where the name is "Not Listed" or empty
func TravellerName(name string) types.PersonName {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "Right Hon. ", "")
	name = strings.ReplaceAll(name, "Hon. ", "")

	if strings.Contains(name, "Not Listed") || name == "" {
		return types.PersonName{
			FirstName: "Not Listed",
			LastName:  "Not Listed",
		}
	}

	// Check if the name contains a comma before splitting
	if strings.Contains(name, ", ") {
		nameArr := strings.Split(name, ", ")
		return types.PersonName{
			FirstName: nameArr[1],
			LastName:  nameArr[0],
		}
	} else {
		// Handle case where name doesn't have a comma
		// Set the entire name as LastName
		return types.PersonName{
			FirstName: "",
			LastName:  name,
		}
	}
}

// City formats the city name for proper punctuation
// Special case in Travel expenses
// The city listed is always in all caps
// Format for proper punctuation
func City(cityName string) string {
	cityName = strings.TrimSpace(cityName)
	if cityName == "" {
		return "Not Provided"
	}

	cityName = strings.ToLower(cityName)
	runes := []rune(cityName)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}

	return string(runes)
}

// Supplier formats the supplier name for proper punctuation
func Supplier(supplier string) string {
	if strings.ContainsAny(supplier, " - ") {
		// English and French version present
		versions := strings.Split(supplier, " - ")
		if len(versions) > 0 {
			supplier = versions[0]
		}
	}
	return supplier
}

// EventType formats the event type for proper punctuation
func EventType(text string) string {
	text = strings.TrimSpace(text)
	removeTitle := strings.ReplaceAll(text, "Type of Event", "")
	removeClaim := strings.ReplaceAll(removeTitle, "Claim", "")
	trimmed := strings.TrimSpace(removeClaim)

	return trimmed
}

// ConvertDateFormat converts a date string from "January 2, 2006" to "2006-01-02"
// Convert date string into PSQL format
func ConvertDateFormat(dateRange types.DateRange) (types.DateRange, error) {

	// Must use same date structure in the new format
	// This ensures that the input format can be formatted correctly
	const inputFormat = "January 2, 2006"
	const newFormat = "2006-01-02"

	formattedStartDate, err := time.Parse(inputFormat, dateRange.StartDate)
	if err != nil {
		return types.DateRange{}, err
	}

	formattedEndDate, err := time.Parse(inputFormat, dateRange.EndDate)
	if err != nil {
		return types.DateRange{}, err
	}

	return types.DateRange{
		StartDate: formattedStartDate.Format(newFormat),
		EndDate:   formattedEndDate.Format(newFormat),
	}, nil
}

// IsTripCancelled checks if the travel purpose indicates a cancelled trip
func IsTripCancelled(travelPurpose string) bool {
	if strings.Contains(travelPurpose, "cancel") {
		return true
	}
	return false
}

// HandleCancelledTrip sets the traveller's date to empty string if the start date is "Not Provided"
func HandleCancelledTrip(traveller *types.Traveller, dates types.DateRange) {
	// Set date to empty string if dates.StartDate is "Not Provided"
	if dates.StartDate == "Not Provided" {
		traveller.Date = "" // This will be converted to NULL in the database
	} else {
		traveller.Date = dates.StartDate
	}
	traveller.DepartureCity = "Not Listed"
	traveller.DestinationCity = "Not Listed"
}

// IsLoungeVisit checks if the travel purpose indicates a visit to the Maple Leaf Lounge
// This is a special case for travel expenses
func IsLoungeVisit(travelPurpose string) bool {
	if strings.Contains(travelPurpose, "Maple Leaf Lounge") {
		return true
	}
	return false
}

// HandleLoungeVisit sets the traveller's date to empty string if the start date is "Not Provided"
// Sets the departure and destination cities to "Not Listed"
// This is a special case for travel expenses
func HandleLoungeVisit(traveller *types.Traveller, dates types.DateRange) {
	// Set date to empty string if dates.StartDate is "Not Provided"
	if dates.StartDate == "Not Provided" {
		traveller.Date = "" // This will be converted to NULL in the database
	} else {
		traveller.Date = dates.StartDate
	}
	traveller.DepartureCity = "Not Listed"
	traveller.DestinationCity = "Not Listed"
}

// CityName formats the city name for proper punctuation
// Handles special cases where French names are used
// Remove extra parenthese and their corresponding text
// This will prevent airport search failure
func CityName(city string) string {

	if strings.Contains(city, "montréal, qc") || strings.Contains(city, "montréal") {
		return "Montreal"
	}

	var formatted []rune
	insideParen := false

	for _, v := range city {
		if v == '(' {
			insideParen = true
			continue
		}

		if v == ')' {
			insideParen = false
			continue
		}

		if !insideParen {
			formatted = append(formatted, v)
		}
	}

	return string(formatted)
}

// IsPostgresDate checks if a date string is in the Postgres format "YYYY-MM-DD"
func IsPostgresDate(dateStr string) bool {
	_, err := time.Parse("2006-01-02", dateStr)
	return err == nil
}

func ReadTravelErrors() (map[string]int, error) {
	locationErrMap := make(map[string]int)
	file, err := os.Open("sample.txt")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if count%2 == 0 { // Only process even-numbered lines (0, 2, 4...)
			fmt.Println(line)
			location := processTravelErr(strings.TrimSpace(line))
			locationErrMap[location]++
		}
		count++
	}
	return locationErrMap, nil
}

func processTravelErr(line string) string {
	splitLine := strings.Split(line, ": ")
	fmt.Println("split line: ", splitLine)

	locationErr := splitLine[4]
	splitErr := strings.Split(locationErr, "for ")

	location := splitErr[1]
	return location
}
