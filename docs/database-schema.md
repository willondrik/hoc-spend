# Database schema and relationships

This document describes the PostgreSQL schema used by `hoc-spend` after the
fresh-install schema and migrations `002` through `010` have been applied. The
authoritative DDL is [`../db/001_initial_schema.sql`](../db/001_initial_schema.sql);
existing databases must also receive each applicable numbered migration in
`db/`.

For a visual overview, see the [Mermaid ERD](../diff/fulldb/erd.md).

## Design overview

The model has three disclosure domains:

- **MP**: a quarterly report contains an MP subject for each disclosed member;
  each subject has contract, hospitality, and travel details.
- **Administration**: a quarterly report owns its detail records directly;
  Administration source pages do not expose a comparable per-person summary.
- **Officer**: a quarterly report contains officer subjects; each subject owns
  contract, hospitality, and travel details.

`people` is the normalized name record used for officers and identifiable
travellers. `mps` is a one-to-one MP extension of `people`, with a durable
internal disclosure identity and optional current House-profile enrichment.

## Relationship map

```text
people ── 0..1 mps ──< mp_affiliations ──< report_subjects >── disclosure_reports
  │                            │                    │
  │                            │                    ├──< MP expense tables
  │                            │                    └──< Officer expense tables
  │                            │
  ├──< mp_travelers >── mp_travel_expenses
  ├──< administration_travel_expenses
  └──< officer_travel_expenses

disclosure_reports ──< Administration contract / hospitality / travel tables
```

`report_subjects` is polymorphic by `subject_kind`:

- `subject_kind = 'mp'`: `mp_affiliation_id` is required and `person_id` is
  null. The person is reached through `mp_affiliations.mp_id -> mps.person_id`.
- `subject_kind = 'officer'`: `person_id` is required and `mp_affiliation_id`
  is null.

The database enforces this rule with
`report_subjects_subject_reference_check`.

## Shared identity and report tables

### `disclosure_reports`

One source quarterly report per disclosure domain.

| Column group | Meaning |
| --- | --- |
| `domain`, `fiscal_year`, `fiscal_quarter` | Durable quarterly report identity. Unique together. `domain` is `mp`, `administration`, or `officer`. |
| `period_start`, `period_end` | Parsed reporting period, when the House page supplies one. |
| `source_url` | Canonical quarterly report URL. Unique. |
| `*_source_url` | Category detail-page URLs. Administration owns these at report level. |
| `*_csv_url` | Optional House CSV-download URLs for category pages. |
| `scraped_at`, `created_at`, `updated_at` | Ingestion and row lifecycle timestamps. |

### `people`

Normalized source names. A `people` row can be an officer, an MP's base person
record, an identifiable traveller, or more than one of those roles. Names are
not globally unique because the sources do not expose stable person IDs.

### `mps`

One row per durable disclosure-backed MP identity, with a one-to-one
relationship to `people` through unique `person_id`.

| Column group | Meaning |
| --- | --- |
| `source_identity` | Required, unique synthetic identity generated from normalized MP name and constituency. It is used to load historical MP reports safely. It is **not** a public URL. |
| `profile_url` | Optional, unique canonical House member-page URL. It is populated only by current-MP profile enrichment. |
| `province`, `avatar`, `email`, `website`, `hill_office`, `constituency_office`, `roles` | Latest profile attributes fetched from the current House member directory/profile page. `avatar` is raw image bytes; offices and roles are JSONB. |
| timestamps | `updated_at` changes when profile enrichment updates the row. |

Current House profiles are matched conservatively by normalized **name + current
constituency**. Historical/former MPs may therefore retain a null `profile_url`
and empty profile fields.

### `mp_affiliations`

Historical MP disclosure context. An MP can have multiple affiliation rows for
different constituency/caucus combinations. `province` is the riding's
province/territory as resolved during report loading from the version-controlled
`pkg/scraper/load/constituency_provinces.csv` reference; it is deliberately
separate from `mps.province`, which is optional current-profile enrichment. An
unknown constituency is a loader error, rather than a nullable analytics value.
The unique key is `(mp_id,
constituency, caucus)`. `effective_from` and `effective_to` are available for
temporal context when source data supports it.

### `report_subjects`

The source summary row for an MP or House Officer in a quarterly report.

| Column group | Meaning |
| --- | --- |
| identity | `report_id` plus either `mp_affiliation_id` (MP) or `person_id` (officer). Unique constraints prevent duplicate subjects within a report. |
| summary values | `salaries`, `travel_total`, `hospitality_total`, and `contract_total` are the House summary totals, not recomputed values. |
| source links | `source_url`, category `*_source_url`, and `*_csv_url` identify the pages used to parse this subject's detail records. |
| `caucus`, `role` | Source summary metadata. |

## MP expense tables

All MP detail tables ultimately belong to an MP `report_subjects` row. Their
source rows are additive: a retry does not overwrite existing detail rows.

Contract `category` values are loader-derived, broad filters. The `advertising`
value is intentionally strict: it requires source-description wording that
explicitly identifies advertising or publicity. It is useful for an auditable
proxy analysis but is not an official House advertising-account total.

| Table | Parent | Key data and uniqueness |
| --- | --- | --- |
| `mp_contract_expenses` | `report_subjects` | Supplier, description, category, dates, total, refund flag, source URL. Unique `(report_subject_id, source_key)`. |
| `mp_hospitality_expenses` | `report_subjects` | Event date/location/attendance/purpose, total, cost per person, refund flag. Unique `(report_subject_id, source_key)`. |
| `mp_hospitality_expense_items` | `mp_hospitality_expenses` | Itemized claim, supplier, and cost. Unique `(hospitality_expense_id, source_key)`. |
| `mp_travel_expenses` | `report_subjects` | Date range, cost components, total, flight points, duplicate/refund flags, source URL. Unique `(report_subject_id, sequence)`. |
| `mp_travelers` | `mp_travel_expenses` | Traveller role/purpose/date/route/mode and optional `people` reference. Unique `(travel_expense_id, source_key)`. Deleting a person sets `person_id` to null rather than deleting travel history. |

## Administration expense tables

Administration detail pages apply to a quarterly report, so every Administration
expense references `disclosure_reports` directly instead of `report_subjects`.

| Table | Parent | Key data and uniqueness |
| --- | --- | --- |
| `administration_contract_expenses` | `disclosure_reports` | Contract dates/description/category/total/source URL. Unique `(report_id, source_key)`. |
| `administration_contract_suppliers` | contract expense | One or more suppliers. Unique `(contract_expense_id, supplier)`. |
| `administration_hospitality_expenses` | `disclosure_reports` | House `event_id`, date/location/attendance/purpose/total/source URL. Unique `(report_id, event_id)`. |
| `administration_hospitality_suppliers` | hospitality expense | Supplier and cost. Unique `(hospitality_expense_id, supplier)`. |
| `administration_travel_expenses` | `disclosure_reports` | Cost components, trip metadata, optional traveller `people` reference, source URL. Unique `(report_id, source_key)`. |
| `administration_travel_places` | travel expense | Ordered place list. Unique `(travel_expense_id, position)`. |

The Administration travel source key is occurrence-aware because the House can
reuse one published event ID within a report.

## Officer expense tables

Officer detail tables are owned by an officer `report_subjects` row.

| Table | Parent | Key data and uniqueness |
| --- | --- | --- |
| `officer_contract_expenses` | `report_subjects` | Supplier, description, category, dates, total, source URL. Unique `(report_subject_id, source_key)`. |
| `officer_hospitality_expenses` | `report_subjects` | Date/location/attendance/purpose/total/cost per person, source URL. Unique `(report_subject_id, source_key)`. |
| `officer_hospitality_suppliers` | hospitality expense | Supplier and cost. Unique `(hospitality_expense_id, supplier, cost)`. |
| `officer_travel_expenses` | `report_subjects` | Date range, cost components, total, purpose, optional traveller `people` reference, source URL. Unique `(report_subject_id, source_key)`. |
| `officer_travel_places` | travel expense | Ordered place list. Unique `(travel_expense_id, position)`. |

## Referential actions

| Parent relationship | Action | Rationale |
| --- | --- | --- |
| report/subject/expense parent to dependent source details | `ON DELETE CASCADE` | Removing a source report or detail parent removes only children that cannot exist independently. |
| `mps.person_id -> people.id` | `ON DELETE CASCADE` | An MP extension has no meaning without its base person record. |
| `mp_affiliations.mp_id`, report-subject identity references | `ON DELETE RESTRICT` | Prevent accidental deletion of a durable identity referenced by disclosure history. |
| traveller person references | `ON DELETE SET NULL` | Preserve expense history if a normalized traveller identity is removed or repaired. |

## Derived views

### `report_subject_expense_reconciliation`

Compares each MP or officer subject's summary totals with sums of the detail
tables. It provides category totals, deltas, and boolean mismatch flags for
contract, hospitality, and travel. Use it to identify parser discrepancies or
reports that need a re-scrape.

```sql
SELECT *
FROM report_subject_expense_reconciliation
WHERE contract_mismatch OR hospitality_mismatch OR travel_mismatch
ORDER BY fiscal_year DESC, fiscal_quarter DESC, subject_kind, last_name, first_name;
```

### `person_expense_totals`

Provides person-centric totals in separate role columns:

- `mp_subject_total`
- `officer_subject_total`
- `mp_traveller_total`
- `officer_traveller_total`
- `administration_traveller_total`

Do not sum these columns blindly: a subject's own travel can be present in both
their subject disclosure and a traveller row, which would double-count it.

## Loading and re-scraping rules

- A complete report load is transactional. If parsing or loading fails, that
  report's changes roll back.
- Detail records are idempotent and additive, using source-scoped unique keys.
- MP and officer `report_subjects` rows are refreshed on complete re-scrapes so
  source summary totals and category URLs can be corrected.
- Current MP profile enrichment is separate from historical disclosure loading.
  A successful MP-inclusive `-apply` runs enrichment after report loading;
  `-enrich-mp-profiles` reruns only that pass.
- A database created before the current bootstrap must apply all migrations,
  especially `010_add_mp_source_identities.sql`,
  `011_add_mp_affiliation_provinces.sql`, and
  `012_add_advertising_contract_category.sql`; the current MP loader requires
  `mps.source_identity` and a resolved historical affiliation province.

## Operational checks

Count current MP profile enrichment:

```sql
SELECT
    count(*) AS mps,
    count(profile_url) AS canonical_profile_urls,
    count(*) FILTER (WHERE avatar IS NOT NULL AND octet_length(avatar) > 0) AS avatars,
    count(*) FILTER (WHERE roles <> '[]'::jsonb) AS profiles_with_roles
FROM mps;
```

Inspect a profile with its underlying source person:

```sql
SELECT
    m.id,
    p.first_name,
    p.last_name,
    m.source_identity,
    m.profile_url,
    m.email,
    m.roles,
    m.updated_at
FROM mps AS m
JOIN people AS p ON p.id = m.person_id
ORDER BY m.id;
```