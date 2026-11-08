# hoc-spend

`hoc-spend` is a Go library for retrieving and parsing publicly disclosed House of Commons of Canada spending data from [ourcommons.ca](https://www.ourcommons.ca/). It is intended for applications that need structured data from the House of Commons proactive-disclosure pages.

The repository provides reusable HTTP, HTML extraction, transformation, normalization, domain-model, and incremental PostgreSQL-loading packages, plus a PostgreSQL bootstrap schema and a frontend data-contract document. It **does not yet provide a complete command-line scraper, an HTTP API, or built-in JSON/CSV export workflow**: [`cmd/scraper/main.go`](./cmd/scraper/main.go) is a placeholder. Consumers are expected to compose the library packages and persist or serialize the returned structs as appropriate.

## What it covers

The scraper supports the three proactive-disclosure report domains:

- **Members of Parliament (MPs)** — quarterly report discovery, summary data, and detailed contract, hospitality, and travel expenses.
- **House Administration** — quarterly report discovery and detailed contract, hospitality, and travel expenses.
- **House Officers** — quarterly report discovery, summary data, and detailed contract, hospitality, and travel expenses.

It also includes helpers for retrieving and parsing MP profile information.

## How it works

At a high level, an integration follows this sequence:

1. Create a `requests.RequestManager`. It uses a 30-second HTTP timeout and retries retryable failures (HTTP 429 and 5xx responses) up to five times.
2. Create an `extract.ExtractSvc` and fetch one of the House of Commons disclosure index pages.
3. Pass the response to the relevant report-discovery transformer:
   - `transform.MpExpenditureReports`
   - `transform.AdministratorExpenditureReports`
   - `transform.OfficerExpenditureReports`
4. Fetch a report page and parse the report summary or the category links using the relevant transformer.
5. Fetch each category page and pass a `*goquery.Document` to the detailed expense parser.
6. Serialize, store, or otherwise consume the returned types from `pkg/scraper/types`.

For MP reports, `ExtractSvc.AllMpExpenses` can retrieve the contract, hospitality, and travel URLs concurrently after they have been discovered.

### Data transformation

The parser adapts the House of Commons HTML tables into typed Go structs. It includes logic for:

- Discovering reporting periods, fiscal years, and fiscal quarters.
- Constructing absolute URLs from relative House of Commons links.
- Parsing Canadian currency with exact decimal arithmetic (`shopspring/decimal`).
- Identifying negative/refunded expense entries.
- Calculating hospitality cost per attendee, rounded to cents; it returns zero when attendance is missing or not positive.
- Parsing nested hospitality and travel detail rows, including travellers, destinations, cost components, and flight points.
- Normalizing selected supplier-name variants through the generated map in `pkg/scraper/utils/supplier_names.go`.
- Handling known travel special cases such as cancelled trips and Maple Leaf Lounge entries.

The scraper depends on the markup published by `ourcommons.ca`; markup or disclosure-format changes may require selector and parsing updates.

## Project layout

```text
cmd/scraper/                  Placeholder command package; no runnable CLI yet
db/001_initial_schema.sql     psql bootstrap that creates the hoc_spend database and its schema
docs/frontend-data-contract.md Client-facing serialized API-shape contract
pkg/scraper/extract/          House of Commons HTTP source endpoints and concurrent MP detail retrieval
pkg/scraper/requests/         HTTP client, timeout, retry, and response-status handling
pkg/scraper/transform/        HTML-to-domain-model parsers for MPs, administration, and officers
pkg/scraper/types/            Shared models for reports, summaries, expenses, people, and travel
pkg/scraper/utils/            Date, currency, name, city, supplier, and calculation helpers
```

### Database model

The PostgreSQL bootstrap separates generic named participants from durable MP
records. `people` stores source names and is used for travellers and officers;
`mps` has a one-to-one link to `people` and owns the latest MP-only profile
enrichment (profile URL, avatar, contact details, offices, and roles).
`mp_affiliations` records each MP's historical riding and caucus combination,
uniquely keyed by MP, riding, and caucus. MP quarterly `report_subjects` rows
reference an affiliation, so every expense remains attributable to one MP while
also retaining the riding and caucus in effect for that disclosure. Officer
subjects and all traveller references continue to use `people`.

## Requirements

- **Go 1.25.0 or later** — the module declares `go 1.25.0` in [`go.mod`](./go.mod).
- Network access to `https://www.ourcommons.ca` when fetching live disclosure data.
- **PostgreSQL and `psql`** — required only when initializing or using the included database schema.

No Docker runtime, JavaScript runtime, or API credentials are required by the current implementation. A database is optional for using the scraper library, but PostgreSQL is required if you want to initialize the included schema.

## Dependencies

Direct dependencies are managed with Go modules:

| Dependency | Purpose |
| --- | --- |
| [`github.com/PuerkitoBio/goquery`](https://github.com/PuerkitoBio/goquery) | HTML parsing and CSS-selector queries |
| [`github.com/shopspring/decimal`](https://github.com/shopspring/decimal) | Exact decimal representation and arithmetic for money |

Transitive dependencies are recorded in [`go.sum`](./go.sum). Use `go mod download` to install the complete module dependency set.

## Setup

```bash
git clone https://github.com/willondrik/hoc-spend.git
cd hoc-spend
go mod download
```

Verify the environment and compile all packages:

```bash
go version
go build ./...
```

### PostgreSQL database setup

[`db/001_initial_schema.sql`](./db/001_initial_schema.sql) is a single pasteable fresh-install script. Open `psql` as a role that can create databases:

```bash
sudo -u postgres psql -d postgres
```

At the resulting `postgres=#` prompt, paste the complete contents of `db/001_initial_schema.sql`. The file runs:

```sql
CREATE DATABASE hoc_spend;
\c hoc_spend
```

and then creates the normalized reports, people, expense, supplier, traveller, and index records required by the scraper domains. The `\c hoc_spend` line is a `psql` command and must remain on its own line.

Your PostgreSQL role must be able to create databases. The command respects normal libpq settings such as `PGHOST`, `PGPORT`, `PGUSER`, and `PGPASSWORD`.

This is a **fresh-install script** and intentionally fails if `hoc_spend` already exists. Drop `hoc_spend` before pasting it again. The repository does not yet include a migration runner for changing an existing database.

#### Hosted database connection

The Go database package accepts a complete PostgreSQL connection URL from
`DATABASE_URL` (preferred) or `DB_URL`. This is the intended configuration for
DigitalOcean Managed Databases because the URL keeps the hosted service's TLS
settings and safely encodes credentials. For example, retain the
`sslmode=require` query parameter supplied by DigitalOcean:

```bash
DATABASE_URL='postgresql://user:password@host:25060/defaultdb?sslmode=require'
```

`db.NewContext(ctx)` parses the URL and calls `Ping` before returning a pool;
it fails fast instead of allowing a scraper run to begin with an unreachable
database. Do not commit this connection URL. The repository's `.env` file is
ignored by Git.

#### Incremental batch loading

`pkg/scraper/load` provides an additive loader for source identities already
represented in the schema:

- `Loader.InsertNewMPProfiles(ctx, profiles)` accepts `[]*types.MPDetails` and
  treats `ProfileURL` as the durable MP identity. It batch-checks
  `mps.profile_url`, then inserts only missing profiles and their required
  `people` rows. It intentionally does not match profiles by name, caucus, or
  constituency, because those values can be ambiguous or change over time.
- `Loader.NewReports(ctx, domain, reports)` is the read-only preflight for a
  discovered report list. It returns only periods not already hosted, so the
  caller can avoid fetching their detail pages. `Loader.InsertNewReports(ctx,
  domain, reports)` uses the same matching rules and inserts only unseen report
  rows.

Both methods deduplicate the supplied slice before querying, write in a
transactional pgx batch, and use `ON CONFLICT DO NOTHING` as a final guard for
overlapping scraper jobs. They never issue updates, so routine loads add newly
published records rather than rewriting already-hosted data. A caller should
run report discovery, call `NewReports`, and fetch detail pages only for the
returned periods.

#### Complete report-detail loaders

The loader also has one-transaction APIs for complete parsed quarterly reports:

- `Loader.LoadAdministrationReport(ctx, types.AdminReport)`
- `Loader.LoadOfficerReport(ctx, report, []load.OfficerReportSubject)`
- `Loader.LoadMPReport(ctx, report, []load.MPReportSubject)`

Each persists its report header, subjects where applicable, expenses, and
supplier/place/traveller children in the same transaction. The detail loaders
are additive and never update an existing source row. A failed load rolls back
the whole report transaction, allowing a later run to retry it cleanly.

For MP report subjects, `MPReportSubject.ProfileURL` is required and must
already exist in `mps.profile_url`; name-only MP matching is intentionally not
supported. Load MP profiles first with `InsertNewMPProfiles`.

Most source detail rows receive a deterministic SHA-256 `source_key`, scoped
to their parent report or subject. This protects against duplicate writes if a
report is retried after an interruption. Administration hospitality/travel use
their House-provided `event_id`; MP travel uses the parsed source sequence.

If the database was created using an older version of the bootstrap schema,
apply [`db/002_add_idempotent_load_keys.sql`](./db/002_add_idempotent_load_keys.sql)
once before invoking the detail loaders. A new local database initialized with
`db/001_initial_schema.sql` already has these columns and constraints.

The expected client/API payloads are documented in [`docs/frontend-data-contract.md`](./docs/frontend-data-contract.md).

### Data and frontend contract

The PostgreSQL schema is normalized for ingestion and querying; it is not intended to be returned directly to a browser. An API layer should map database `snake_case` fields into the serialized `camelCase` contracts in [`docs/frontend-data-contract.md`](./docs/frontend-data-contract.md).

The contract documents report discovery, MP, House Administration, House Officer, and optional MP-profile shapes. It also records source-data limitations such as missing dates, missing attendance, refunds, and duplicate travel rows.

## Using the library

The packages are designed to be composed by a host application. The following example discovers available MP expenditure reports:

```go
package main

import (
	"fmt"
	"log"

	"github.com/willondrik/hoc-spend/pkg/scraper/extract"
	"github.com/willondrik/hoc-spend/pkg/scraper/requests"
	"github.com/willondrik/hoc-spend/pkg/scraper/transform"
)

func main() {
	requestManager := requests.NewRequestManager()
	service := extract.NewSvc(requestManager)

	response, err := service.MpReports()
	if err != nil {
		log.Fatal(err)
	}
	defer response.Body.Close()

	reports, err := transform.MpExpenditureReports(response)
	if err != nil {
		log.Fatal(err)
	}

	for _, report := range reports.Reports {
		fmt.Printf("FY %d Q%d: %s\n", report.FiscalYear, report.FiscalQuarter, report.URL)
	}
}
```

### Response-body ownership

`RequestManager.Get` returns an `*http.Response`. The caller owns the response body and must call `Close` after parsing it. This also applies to responses returned from `ExtractSvc` methods, including the three responses inside `types.RawExpenseResponses`.

### Output models

Models in `pkg/scraper/types` include JSON tags for most public data structures, so callers can marshal them with Go's standard `encoding/json` package. Monetary values are represented by `types.Money` (an alias of `decimal.Decimal`) and are configured to marshal as JSON numbers rather than quoted strings.

## Configuration

The scraper parsers have no required environment variables. PostgreSQL settings
are required only when using the optional database package or loader.

| Variable | Effect |
| --- | --- |
| `PC_SCRAPER_DEBUG_HOSP` | Enables verbose diagnostic output while parsing MP hospitality tables when set to any non-empty value other than `0`. |
| `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD` | Standard PostgreSQL/libpq connection settings used by `psql` when running `db/001_initial_schema.sql`. |
| `DATABASE_URL` | Preferred complete PostgreSQL URL used by `db.New` / `db.NewContext`; retain DigitalOcean TLS options such as `sslmode=require`. |
| `DB_URL` | Fallback complete PostgreSQL URL when `DATABASE_URL` is unset. |

For example:

```bash
PC_SCRAPER_DEBUG_HOSP=1 go test ./pkg/scraper/transform
```

## Testing and development

Run all tests:

```bash
go test ./...
```

Run a specific package's tests:

```bash
go test ./pkg/scraper/transform
go test ./pkg/scraper/utils
```

Format Go source before committing changes:

```bash
gofmt -w cmd pkg
```

## Current limitations

- There is no wired command-line entry point; `go run ./cmd/scraper` is not a supported way to run a complete scrape.
- The incremental loader currently persists MP profile identities and disclosure-report index rows. Loading report subjects and detailed expense rows is the next persistence step.
- There is no implemented HTTP API; `docs/frontend-data-contract.md` defines the recommended serialized contract for one.
- House Administration summary parsing (`transform.Admins`) is currently a stub; its detailed expense parsers are implemented.
- HTML selectors target the current House of Commons disclosure pages and need maintenance if their HTML structure changes.
- Some parsing helpers contain diagnostic output or edge-case handling intended for ongoing scraper development.

## Contributing

When adding or changing a parser:

1. Preserve the existing domain types and exact-decimal handling for monetary values.
2. Add fixture-based tests for new or changed table layouts and edge cases.
3. Run `gofmt -w` on changed Go files.
4. Run `go test ./...` before opening a change.
5. When changing persistence or API-facing data, update `db/001_initial_schema.sql`, `docs/frontend-data-contract.md`, and this README together.
6. Document any new environment variables, persistence integrations, or runnable commands here.