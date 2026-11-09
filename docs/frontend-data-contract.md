# Frontend data contract

This document defines the **recommended serialized API shapes** a client should consume from `hoc-spend` data. It is derived from the parsed outputs in [`pkg/scraper/types/types.go`](../pkg/scraper/types/types.go), not from raw database rows.

The repository includes a normalized PostgreSQL bootstrap schema in [`db/001_initial_schema.sql`](../db/001_initial_schema.sql), but it does **not** yet include an HTTP API or Go persistence layer. When one is implemented, it should map SQL `snake_case` columns to the `camelCase` names below rather than expose database rows directly.

## Scope and conventions

```ts
/** A Canadian-dollar amount. Use decimal/currency formatting, never binary math. */
type Money = number;

/** A normalized calendar date in YYYY-MM-DD form. */
type ISODate = string;

/** A source date that could not be normalized; display as supplied. */
type SourceDate = string;

type DisclosureDomain = "mp" | "administration" | "officer";

interface DateRange {
  startDate: ISODate | SourceDate;
  endDate: ISODate | SourceDate;
}

interface PersonName {
  firstName: string;
  lastName: string;
}

interface ExpenseCategory {
  totalAmount: Money;
  /** Official House of Commons detail-report URL. */
  url: string;
}

interface ExpenditureReport {
  dateRange: DateRange;
  fiscalYear: number;
  fiscalQuarter: 1 | 2 | 3 | 4;
  /** Official House of Commons quarterly-report URL. */
  url: string;
}
```

### Serialization rules

- All money values are CAD and are serialized as JSON numbers. Format them using a currency formatter such as `Intl.NumberFormat("en-CA", { style: "currency", currency: "CAD" })`.
- A negative amount is a refund/reversal. For MP entries, `isRefunded` is also supplied to make this explicit.
- `DateRange` source values are normally `YYYY-MM-DD`; some House Administration source values can remain display text if the source is not parseable. Clients must not assume arbitrary non-empty strings are ISO dates.
- An empty string means the source did not provide a value. Arrays are returned as `[]`, not `null`.
- Fields marked `?` may be omitted. Fields typed `null` are deliberately present but unknown/unavailable.
- Database identifiers are internal. Do not make the frontend depend on their exact type or expose them unless the API deliberately adds public IDs.

## Shared API envelopes

```ts
interface PageMeta {
  limit: number;
  offset: number;
  total: number;
}

interface ListResponse<T> {
  data: T[];
  meta: PageMeta;
}

interface DetailResponse<T> {
  data: T;
}
```

Use `ListResponse<T>` for report, subject, and expense-list endpoints. Use `DetailResponse<T>` when a screen needs a single subject/expense and its nested children. These are contract recommendations, not endpoints currently implemented by this repository.

## Report discovery

```ts
interface ReportListItem extends ExpenditureReport {
  domain: DisclosureDomain;
}

interface ReportListResponse {
  data: ReportListItem[];
}
```

Each report represents one fiscal quarter in one House of Commons disclosure domain. `fiscalYear` is derived from the reporting period’s start year by the scraper.

## MP summaries and detail

```ts
interface MpExpenseSummary {
  name: PersonName;
  constituency: string;
  caucus: string;
  salaries: Money;
  travelExpenses: ExpenseCategory;
  hospitalityExpenses: ExpenseCategory;
  contractExpenses: ExpenseCategory;
  reportingPeriod: DateRange;
  fiscalYear: number;
  fiscalQuarter: 1 | 2 | 3 | 4;
  /** The source currently does not populate this consistently. */
  url: string;
}

interface ContractExpense {
  supplier: string;
  description: string;
  /** Normalized when possible; otherwise the original source value. */
  date: ISODate | SourceDate;
  total: Money;
  /** Omitted or false for a non-refund entry. */
  isRefunded?: boolean;
}

interface ExpenseItem {
  claim: string;
  supplier: string;
  cost: Money;
}

interface HospitalityEvent {
  type: string;
  expenseItems: ExpenseItem[];
}

interface HospitalityExpense {
  date: ISODate | SourceDate;
  location: string;
  attendance: number;
  purpose: string;
  event: HospitalityEvent;
  totalCost: Money;
  /** Zero when attendance is unavailable or less than one. */
  costPerPerson: Money;
  isRefunded?: boolean;
}

interface TravelData {
  distance: number;
  distanceUnit: string;
  emissions: number;
  emissionsUnit: string;
  class: string;
}

interface Traveller {
  name: PersonName;
  type: string;
  purpose: string;
  date: ISODate | SourceDate;
  departureCity: string;
  destinationCity: string;
  travelData: TravelData;
  /** Usually "Air" or "Ground"; do not assume this is an exhaustive enum. */
  transportationMode: string;
}

interface TravelCosts {
  transportation: Money;
  accommodation: Money;
  mealsAndIncidentals: Money;
  total: Money;
}

interface FlightPoints {
  regular: number;
  special: number;
  usa: number;
}

interface TravelExpense {
  claim: string;
  sequence?: number;
  isDuplicate?: boolean;
  isRefunded?: boolean;
  dateRange: DateRange;
  travellers: Traveller[];
  costs: TravelCosts;
  flightPoints: FlightPoints;
}

interface MpExpenseDetails {
  contractExpenses: ContractExpense[];
  hospitalityExpenses: HospitalityExpense[];
  travelExpenses: TravelExpense[];
}

interface MpExpenseDetailResponse {
  summary: MpExpenseSummary;
  details: MpExpenseDetails;
}
```

`MpExpenseSummary` is for report tables/cards. `MpExpenseDetailResponse` is for a selected MP’s quarterly drill-down. Keeping the summary and arrays separate prevents a field from changing between an `ExpenseCategory` object and an array.

## House Administration detail

The scraper currently has no populated Administration summary parser. The frontend should treat this domain as report-level detail data until that parser is implemented.

```ts
interface AdminContractExpense {
  suppliers: string[];
  description: string;
  contractPeriod: DateRange;
  originalContractValue: Money;
  amendedContractValue: Money;
}

interface AdminHospitalityExpense {
  eventId: string;
  host: string;
  purpose: string;
  date: ISODate | SourceDate;
  totalCost: Money;
  /** Zero when attendance is absent, invalid, or not positive. */
  costPerPerson: Money;
  location: string;
  suppliers: string[];
  /** Preserved as source text because values are not always numeric. */
  attendance: string;
}

interface AdminTravelExpenseBreakdown {
  transportation: Money;
  accommodations: Money;
  mealsAndIncidentals: Money;
  other: Money;
}

interface AdminTravelExpense {
  eventId: string;
  traveller: PersonName;
  purpose: string;
  dateRange: DateRange;
  totalCost: Money;
  placesVisited: string[];
  expenseBreakdown: AdminTravelExpenseBreakdown;
}

interface AdministrationReportDetail extends ExpenditureReport {
  domain: "administration";
  /** Official House of Commons Administration detail-report URLs, when published. */
  categoryUrls: {
    contract: string | null;
    hospitality: string | null;
    travel: string | null;
  };
  contractExpenses: AdminContractExpense[];
  hospitalityExpenses: AdminHospitalityExpense[];
  travelExpenses: AdminTravelExpense[];
}
```

`eventId` is the House Administration’s source event identifier and is the preferred display/reference value for an Administration event.
`categoryUrls` is nullable per category because the House can publish no detail link for a zero-spend category. In persistence, these Administration links are report-level fields; Officer category URLs remain subject-level because they vary by officer.

## House Officer summaries and detail

```ts
interface OfficerExpenseSummary {
  name: PersonName;
  role: string;
  caucus: string;
  salaries: Money;
  travelExpenses: ExpenseCategory;
  hospitalityExpenses: ExpenseCategory;
  contractExpenses: ExpenseCategory;
  reportingPeriod: DateRange;
  fiscalYear: number;
  fiscalQuarter: 1 | 2 | 3 | 4;
  url: string;
}

interface OfficerContractExpense {
  supplier: string;
  description: string;
  date: ISODate | SourceDate;
  totalCost: Money;
}

interface OfficerSupplier {
  name: string;
  cost: Money;
}

interface OfficerHospitalityExpense {
  date: ISODate | SourceDate;
  location: string;
  totalAttendance: number;
  purpose: string;
  totalCost: Money;
  costPerPerson: Money;
  suppliers: OfficerSupplier[];
}

interface OfficerTravelExpense {
  period: DateRange;
  departureCity: string;
  placesVisited: string[];
  transportationCost: Money;
  accommodationCost: Money;
  mealsAndIncidentals: Money;
  totalCost: Money;
  purpose: string;
  traveller: PersonName;
}

interface OfficerExpenseDetails {
  contractExpenses: OfficerContractExpense[];
  hospitalityExpenses: OfficerHospitalityExpense[];
  travelExpenses: OfficerTravelExpense[];
}

interface OfficerExpenseDetailResponse {
  summary: OfficerExpenseSummary;
  details: OfficerExpenseDetails;
}
```

`OfficerExpenseDetailResponse` likewise keeps a summary category total separate from its detailed arrays.

## Optional MP profile enrichment

MP profile data is fetched from a different House of Commons page and is optional enrichment, not a prerequisite for expense screens.

```ts
interface OfficeAddress {
  name: string;
  institution: string;
  streetAddress: string;
  city: string;
  province: string;
  country: string;
  postalCode: string;
  phoneNumber: string;
  faxNumber: string;
  description: string;
}

interface MpContactInfo {
  name: PersonName;
  email: string;
  website: string;
  hillOffice: OfficeAddress;
  constituencyOffice: OfficeAddress;
}

interface MpProfileDetail {
  name: PersonName;
  province: string;
  contact: MpContactInfo;
  roles: string[];
  /** Serve as a separate image endpoint or data URL; do not put raw bytes in JSON. */
  avatarUrl: string | null;
  profileUrl: string;
}
```

## Recommended endpoint-to-shape mapping

| Endpoint purpose | Response shape |
| --- | --- |
| List periods by disclosure domain | `ReportListResponse` |
| List MPs for a period | `ListResponse<MpExpenseSummary>` |
| MP quarterly drill-down | `DetailResponse<{ summary: MpExpenseSummary; details: { contractExpenses: ContractExpense[]; hospitalityExpenses: HospitalityExpense[]; travelExpenses: TravelExpense[] } }>` |
| Administration quarterly drill-down | `DetailResponse<AdministrationReportDetail>` |
| List officers for a period | `ListResponse<OfficerExpenseSummary>` |
| Officer quarterly drill-down | `DetailResponse<{ summary: OfficerExpenseSummary; details: { contractExpenses: OfficerContractExpense[]; hospitalityExpenses: OfficerHospitalityExpense[]; travelExpenses: OfficerTravelExpense[] } }>` |
| Optional MP bio/contact profile | `DetailResponse<MpProfileDetail>` |

## Known limitations to surface in the UI

1. The House of Commons source can omit dates, attendance, suppliers, traveller fields, and profile data. Render a neutral placeholder instead of treating missing values as errors.
2. `costPerPerson === 0` does not necessarily mean the event was free; it can mean attendance was not available.
3. Travel claim rows can be duplicated in the source; MP records expose `isDuplicate` when detected.
4. The current scraper’s House Administration summary function is a stub. Do not build an Administration per-person summary view until the backend implements it.
5. A source URL points to the official disclosure page and should be safe to expose as an external link.

## Contract maintenance

When the scraper gains, removes, or changes a serialized field, update this document together with the corresponding Go type/parser and the persistence schema in [`db/001_initial_schema.sql`](../db/001_initial_schema.sql). Keep database-oriented names and internal IDs out of client payloads unless the API deliberately promotes them to stable public identifiers.