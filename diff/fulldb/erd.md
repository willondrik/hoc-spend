# hoc-spend database ERD

This Mermaid ERD documents the final physical schema represented by
[`../../db/001_initial_schema.sql`](../../db/001_initial_schema.sql) and
migrations `002` through `010`. It focuses on primary keys, foreign keys, and
the durable source-identity fields that matter for joins. For the complete
column-by-column reference and loading semantics, see
[`../../docs/database-schema.md`](../../docs/database-schema.md).

`report_subject_expense_reconciliation` and `person_expense_totals` are derived
views, so they are documented below rather than represented as ERD entities.

```mermaid
erDiagram
    disclosure_reports {
        BIGINT id PK
        TEXT domain
        INTEGER fiscal_year
        SMALLINT fiscal_quarter
        TEXT source_url UK
        TEXT travel_source_url
        TEXT hospitality_source_url
        TEXT contract_source_url
        TEXT travel_csv_url
        TEXT hospitality_csv_url
        TEXT contract_csv_url
    }

    people {
        BIGINT id PK
        TEXT first_name
        TEXT last_name
    }

    mps {
        BIGINT id PK
        BIGINT person_id FK, UK
        TEXT source_identity UK
        TEXT profile_url UK
        TEXT province
        BYTEA avatar
        TEXT email
        TEXT website
        JSONB hill_office
        JSONB constituency_office
        JSONB roles
    }

    mp_affiliations {
        BIGINT id PK
        BIGINT mp_id FK
        TEXT constituency
        TEXT caucus
        DATE effective_from
        DATE effective_to
    }

    report_subjects {
        BIGINT id PK
        BIGINT report_id FK
        BIGINT person_id FK
        BIGINT mp_affiliation_id FK
        TEXT subject_kind
        TEXT role
        NUMERIC salaries
        NUMERIC travel_total
        NUMERIC hospitality_total
        NUMERIC contract_total
        TEXT source_url
        TEXT travel_source_url
        TEXT hospitality_source_url
        TEXT contract_source_url
        TEXT travel_csv_url
        TEXT hospitality_csv_url
        TEXT contract_csv_url
    }

    mp_contract_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        TEXT source_url
        TEXT supplier
        TEXT category
        NUMERIC total
    }

    mp_hospitality_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        TEXT source_url
        NUMERIC total_cost
    }

    mp_hospitality_expense_items {
        BIGINT id PK
        BIGINT hospitality_expense_id FK
        TEXT source_key
        TEXT supplier
        NUMERIC cost
    }

    mp_travel_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_url
        INTEGER sequence
        NUMERIC total_cost
    }

    mp_travelers {
        BIGINT id PK
        BIGINT travel_expense_id FK
        BIGINT person_id FK
        TEXT source_key
    }

    administration_contract_expenses {
        BIGINT id PK
        BIGINT report_id FK
        TEXT source_key
        TEXT source_url
        TEXT category
        NUMERIC total_cost
    }

    administration_contract_suppliers {
        BIGINT id PK
        BIGINT contract_expense_id FK
        TEXT supplier
    }

    administration_hospitality_expenses {
        BIGINT id PK
        BIGINT report_id FK
        TEXT event_id
        TEXT source_url
        NUMERIC total_cost
    }

    administration_hospitality_suppliers {
        BIGINT id PK
        BIGINT hospitality_expense_id FK
        TEXT supplier
    }

    administration_travel_expenses {
        BIGINT id PK
        BIGINT report_id FK
        BIGINT traveller_person_id FK
        TEXT source_key
        TEXT source_url
        NUMERIC total_cost
    }

    administration_travel_places {
        BIGINT id PK
        BIGINT travel_expense_id FK
        SMALLINT position
        TEXT place
    }

    officer_contract_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        TEXT source_url
        TEXT category
        NUMERIC total_cost
    }

    officer_hospitality_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        TEXT source_url
        NUMERIC total_cost
    }

    officer_hospitality_suppliers {
        BIGINT id PK
        BIGINT hospitality_expense_id FK
        TEXT supplier
    }

    officer_travel_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        BIGINT traveller_person_id FK
        TEXT source_key
        TEXT source_url
        NUMERIC total_cost
    }

    officer_travel_places {
        BIGINT id PK
        BIGINT travel_expense_id FK
        SMALLINT position
        TEXT place
    }

    people ||--o| mps : "is durable MP identity"
    mps ||--o{ mp_affiliations : "has disclosure affiliations"
    disclosure_reports ||--o{ report_subjects : "contains subjects"
    people o|--o{ report_subjects : "officer subject"
    mp_affiliations o|--o{ report_subjects : "MP subject"

    report_subjects ||--o{ mp_contract_expenses : "MP contracts"
    report_subjects ||--o{ mp_hospitality_expenses : "MP hospitality"
    mp_hospitality_expenses ||--o{ mp_hospitality_expense_items : "itemized claims"
    report_subjects ||--o{ mp_travel_expenses : "MP travel"
    mp_travel_expenses ||--o{ mp_travelers : "travellers"
    people o|--o{ mp_travelers : "identified traveller"

    disclosure_reports ||--o{ administration_contract_expenses : "administration contracts"
    administration_contract_expenses ||--o{ administration_contract_suppliers : "suppliers"
    disclosure_reports ||--o{ administration_hospitality_expenses : "administration hospitality"
    administration_hospitality_expenses ||--o{ administration_hospitality_suppliers : "suppliers"
    disclosure_reports ||--o{ administration_travel_expenses : "administration travel"
    people o|--o{ administration_travel_expenses : "identified traveller"
    administration_travel_expenses ||--o{ administration_travel_places : "places"

    report_subjects ||--o{ officer_contract_expenses : "officer contracts"
    report_subjects ||--o{ officer_hospitality_expenses : "officer hospitality"
    officer_hospitality_expenses ||--o{ officer_hospitality_suppliers : "suppliers"
    report_subjects ||--o{ officer_travel_expenses : "officer travel"
    people o|--o{ officer_travel_expenses : "identified traveller"
    officer_travel_expenses ||--o{ officer_travel_places : "places"
```

## Derived views

- `report_subject_expense_reconciliation` compares the MP and officer summary
  totals recorded on `report_subjects` to sums of their loaded contract,
  hospitality, and travel detail records.
- `person_expense_totals` exposes MP-subject, officer-subject, MP-traveller,
  officer-traveller, and Administration-traveller totals in separate columns so
  consumers do not accidentally double-count a subject's own travel.