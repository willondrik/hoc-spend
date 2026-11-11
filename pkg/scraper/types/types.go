package types

import (
	"net/http"

	"github.com/PuerkitoBio/goquery"
	"github.com/shopspring/decimal"
)

// ============================================================
// CORE / SHARED TYPES
// ============================================================

type ExpenseStore struct {
	Admin   []AdminExpenses
	MP      []MPExpenses
	Officer []OfficerExpenses
}

// Money represents a currency value with exact decimal precision.
type Money = decimal.Decimal

func init() {
	decimal.MarshalJSONWithoutQuotes = true
}

type DateRange struct {
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type PersonName struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type ExpenseCategory struct {
	TotalAmount Money  `json:"totalAmount"`
	URL         string `json:"url"`
}

type ExpenseURLs struct {
	Contract    string `json:"contract"`
	Hospitality string `json:"hospitality"`
	Travel      string `json:"travel"`
}

type RawExpenseResponses struct {
	Contract    *http.Response
	Hospitality *http.Response
	Travel      *http.Response
}

type HTMLDocument = *goquery.Document

// ============================================================
// POLLING TYPES
// ============================================================

type PollingResults struct {
	AdminResults   []PollingResult
	MPResults      []PollingResult
	OfficerResults []PollingResult
}

type PollingResult struct {
	Report ExpenditureReport
	IsNew  bool
}

// ============================================================
// REPORT TYPES
// ============================================================

// Generic report list container.
type ExpenditureReports struct {
	Reports []ExpenditureReport
}

type ExpenditureReport struct {
	DateRange     DateRange `json:"dateRange"`
	FiscalYear    int       `json:"fiscalYear"`
	FiscalQuarter int       `json:"fiscalQuarter"`
	URL           string    `json:"url"`
}

// NOTE: We previously had a ReportDomain type to key maps during report discovery.
// Discovery now returns PollingResults directly, so this is no longer needed.

// ============================================================
// MP SUMMARY TYPES
// ============================================================

type MPExpenseSummary struct {
	Name                PersonName      `json:"name"`
	Constituency        string          `json:"constituency"`
	Caucus              string          `json:"caucus"`
	Salaries            Money           `json:"salaries"`
	TravelExpenses      ExpenseCategory `json:"travelExpenses"`
	HospitalityExpenses ExpenseCategory `json:"hospitalityExpenses"`
	ContractExpenses    ExpenseCategory `json:"contractExpenses"`
	ReportingPeriod     DateRange       `json:"reportingPeriod"`
	FiscalYear          int             `json:"fiscalYear"`
	FiscalQuarter       int             `json:"fiscalQuarter"`
	URL                 string          `json:"url"`
}

type MP struct {
	Name            PersonName
	Constituency    string
	Caucus          string
	ReportingPeriod DateRange
	FiscalYear      int
	FiscalQuarter   int
	URL             string
	Expenses        MPExpenses
}

type MPExpenses struct {
	Totals ExpenseTotals

	ContractExpenses       []*ContractExpense
	ContractExpensesURL    string
	ContractExpensesCSVURL string

	HospitalityExpenses       []*HospitalityExpense
	HospitalityExpensesURL    string
	HospitalityExpensesCSVURL string

	TravelExpenses       []*TravelExpense
	TravelExpensesURL    string
	TravelExpensesCSVURL string
}

type MPExpenseResults struct {
	TravelExpenses      []*TravelExpense
	HospitalityExpenses []*HospitalityExpense
	ContractExpenses    []*ContractExpense
	Errors              []error
}

type ExpenseTotals struct {
	Salaries    Money
	Contract    Money
	Hospitality Money
	Travel      Money
	Total       Money
}

// ============================================================
// MP PROFILE TYPES
// ============================================================

type MPDetails struct {
	Name           PersonName
	Province       string
	Contact        MPContactInfo
	Roles          []string
	Avatar         []byte
	ProfileURL     string
	SourceIdentity string
}

// MPDirectoryEntry is one current member listed in the House member directory.
// Constituency is kept with the profile URL so enrichment can safely identify a
// disclosure MP without relying on a name alone.
type MPDirectoryEntry struct {
	Name         PersonName
	Constituency string
	ProfileURL   string
}

type MPProfileURL struct {
	FirstName string
	LastName  string
	URL       string
}

type MPProfile struct {
	FirstName string
	LastName  string
	Province  string
	Contact   MPContactInfo
	Roles     []MPRoleGroup
}

type MPRoleGroup struct {
	CurrentRoles        []Role
	ExecutiveCommittees []Role
}

type Role struct {
	Abbreviation string
	Name         string
}

type MPContactInfo struct {
	Name               PersonName
	Email              string
	Website            string
	HillOffice         OfficeAddress
	ConstituencyOffice OfficeAddress
}

type OfficeAddress struct {
	Name          string
	Institution   string
	StreetAddress string
	City          string
	Province      string
	Country       string
	PostalCode    string
	PhoneNumber   string
	FaxNumber     string
	Description   string
}

// ============================================================
// MP HTML STORAGE
// ============================================================

type MPHTML struct {
	Info            MPExpenseSummary
	ContractHTML    HTMLDocument
	HospitalityHTML HTMLDocument
	TravelHTML      HTMLDocument
}

// ============================================================
// ADMIN TYPES
// ============================================================

type AdminExpenseSummary struct {
	Name                PersonName      `json:"name"`
	Constituency        string          `json:"constituency"`
	Caucus              string          `json:"caucus"`
	Salaries            Money           `json:"salaries"`
	TravelExpenses      ExpenseCategory `json:"travelExpenses"`
	HospitalityExpenses ExpenseCategory `json:"hospitalityExpenses"`
	ContractExpenses    ExpenseCategory `json:"contractExpenses"`
	ReportingPeriod     DateRange       `json:"reportingPeriod"`
	FiscalYear          int             `json:"fiscalYear"`
	FiscalQuarter       int             `json:"fiscalQuarter"`
	URL                 string          `json:"url"`
}

// Admin is the normalized (MP/Officer-like) entity for the House Administration domain.
//
// Naming is intentionally consistent with MP and Officer: the only difference is the
// type prefix.
type Admin struct {
	Name            PersonName
	Role            string
	ReportingPeriod DateRange
	FiscalYear      int
	FiscalQuarter   int
	URL             string
	Expenses        AdminExpenses
}

type AdminExpenses struct {
	TravelExpenses      []AdminTravelExpense
	HospitalityExpenses []AdminHospitalityExpense
	ContractExpenses    []AdminContractExpense

	TravelExpensesURL      string
	HospitalityExpensesURL string
	ContractExpensesURL    string
}

type AdminExpenseURLs struct {
	Travel      string
	Hospitality string
	Contract    string
}

// AdminReport is the per-quarter aggregate scrape output for the House Administration.
// It can optionally be normalized into an Admin via orchestrator.AdminFromQuarterlyReport.
type AdminReport struct {
	Report      ExpenditureReport
	Travel      AdminTravelExpenseCollection
	Contract    AdminContractExpenseCollection
	Hospitality AdminHospitalityExpenseCollection
}

// ============================================================
// ADMIN TRAVEL
// ============================================================

type AdminTravelExpenseCollection struct {
	URL     string
	CSVURL  string
	Entries []AdminTravelExpense
}

type AdminTravelExpense struct {
	EventID          string
	Traveller        PersonName
	Purpose          string
	DateRange        DateRange
	TotalCost        Money
	PlacesVisited    []string
	ExpenseBreakdown AdminTravelExpenseBreakdown
}

type AdminTravelExpenseBreakdown struct {
	Transportation      Money
	Accommodations      Money
	MealsAndIncidentals Money
	Other               Money
}

// ============================================================
// ADMIN HOSPITALITY
// ============================================================

type AdminHospitalityExpenseCollection struct {
	URL     string
	CSVURL  string
	Entries []AdminHospitalityExpense
}

type AdminHospitalityExpense struct {
	EventID       string
	Host          string
	Purpose       string
	Date          string
	TotalCost     Money
	CostPerPerson Money

	Location   string
	Suppliers  []string
	Attendance string
}

// ============================================================
// ADMIN CONTRACTS
// ============================================================

type AdminContractExpenseCollection struct {
	URL     string
	CSVURL  string
	Entries []AdminContractExpense
}

type AdminContractExpense struct {
	Suppliers             []string
	Description           string
	ContractPeriod        DateRange
	OriginalContractValue Money
	AmendedContractValue  Money
}

// ============================================================
// OFFICER TYPES
// ============================================================

type OfficerExpenseSummary struct {
	Name     PersonName `json:"name"`
	Role     string     `json:"role"`
	Caucus   string     `json:"caucus"`
	Salaries Money      `json:"salaries"`

	TravelExpenses      ExpenseCategory `json:"travelExpenses"`
	HospitalityExpenses ExpenseCategory `json:"hospitalityExpenses"`
	ContractExpenses    ExpenseCategory `json:"contractExpenses"`

	ReportingPeriod DateRange `json:"reportingPeriod"`
	FiscalYear      int       `json:"fiscalYear"`
	FiscalQuarter   int       `json:"fiscalQuarter"`
	URL             string    `json:"url"`
}

type Officer struct {
	Name            PersonName
	Role            string
	Caucus          string
	ReportingPeriod DateRange
	FiscalYear      int
	FiscalQuarter   int
	URL             string
	Expenses        OfficerExpenses
}

type OfficerExpenses struct {
	Totals ExpenseTotals

	ContractExpenses       []OfficerContractExpense
	ContractExpensesURL    string
	ContractExpensesCSVURL string

	HospitalityExpenses       []OfficerHospitalityExpense
	HospitalityExpensesURL    string
	HospitalityExpensesCSVURL string

	TravelExpenses       []OfficerTravelExpense
	TravelExpensesURL    string
	TravelExpensesCSVURL string
}

// ============================================================
// OFFICER EXPENSES
// ============================================================

type OfficerTravelExpense struct {
	Period              DateRange  `json:"period"`
	DepartureCity       string     `json:"departureCity"`
	PlacesVisited       []string   `json:"placesVisited"`
	TransportationCost  Money      `json:"transportationCost"`
	AccommodationCost   Money      `json:"accommodationCost"`
	MealsAndIncidentals Money      `json:"mealsAndIncidentals"`
	TotalCost           Money      `json:"totalCost"`
	Purpose             string     `json:"purpose"`
	Traveller           PersonName `json:"traveller"`
}

type OfficerHospitalityExpense struct {
	Date            string            `json:"date"`
	Location        string            `json:"location"`
	TotalAttendance int               `json:"totalAttendance"`
	Purpose         string            `json:"purpose"`
	TotalCost       Money             `json:"totalCost"`
	CostPerPerson   Money             `json:"costPerPerson"`
	Suppliers       []OfficerSupplier `json:"suppliers"`
}

type OfficerSupplier struct {
	Name string `json:"name"`
	Cost Money  `json:"cost"`
}

type OfficerContractExpense struct {
	Supplier    string `json:"supplier"`
	Description string `json:"description"`
	Date        string `json:"date"`
	TotalCost   Money  `json:"totalCost"`
}

// ============================================================
// GENERIC EXPENSE TYPES
// ============================================================

type ContractExpense struct {
	Supplier    string `json:"supplier"`
	Description string `json:"description"`
	Date        string `json:"date"`
	Total       Money  `json:"total"`

	// Negative/refunded entries.
	IsRefunded bool `json:"isRefunded,omitempty"`
}

type HospitalityExpense struct {
	Date          string
	Location      string
	Attendance    int
	Purpose       string
	Event         HospitalityEvent
	TotalCost     Money
	CostPerPerson Money `json:"costPerPerson"`
	IsRefunded    bool  `json:"isRefunded,omitempty"`
}

type HospitalityEvent struct {
	Type         string
	ExpenseItems []ExpenseItem
}

type ExpenseItem struct {
	Claim    string
	Supplier string
	Cost     Money
}

// ============================================================
// TRAVEL TYPES
// ============================================================

type TravelExpense struct {
	Claim        string       `json:"claim"`
	Sequence     int          `json:"sequence,omitempty"`
	IsDuplicate  bool         `json:"isDuplicate,omitempty"`
	IsRefunded   bool         `json:"isRefunded,omitempty"`
	DateRange    DateRange    `json:"dateRange"`
	Travellers   []Traveller  `json:"travellers"`
	Costs        TravelCosts  `json:"costs"`
	FlightPoints FlightPoints `json:"flightPoints"`
}

type TravelCosts struct {
	Transportation      Money `json:"transportation"`
	Accommodation       Money `json:"accommodation"`
	MealsAndIncidentals Money `json:"mealsAndIncidentals"`
	Total               Money `json:"total"`
}

type FlightPoints struct {
	Regular float64 `json:"regular"`
	Special float64 `json:"special"`
	USA     float64 `json:"usa"`
}

type Traveller struct {
	Name               PersonName `json:"name"`
	Type               string     `json:"type"`
	Purpose            string     `json:"purpose"`
	Date               string     `json:"date"`
	DepartureCity      string     `json:"departureCity"`
	DestinationCity    string     `json:"destinationCity"`
	TravelData         TravelData `json:"travelData"`
	TransportationMode string     `json:"transportationMode"`
}

type TravelData struct {
	Distance      float64
	DistanceUnit  string
	Emissions     float64
	EmissionsUnit string
	Class         string
}

// ============================================================
// TASK / ORCHESTRATION TYPES
// ============================================================

type ExtractionTask struct {
	Type               string
	URL                string
	ExtractFromElement string
}

type ExtractionTasks struct {
	Contract    ExtractionTask
	Hospitality ExtractionTask
	Travel      ExtractionTask
}

type ExtractionTaskErrors struct {
	Contract    bool
	Hospitality bool
	Travel      bool
}

type RetryTask struct {
	MPHTML     MPHTML
	Tasks      ExtractionTasks
	TaskErrors ExtractionTaskErrors
}
