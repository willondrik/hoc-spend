# hoc-spend database ERD

This Mermaid ERD documents the final physical schema in [`schema.sql`](./schema.sql), consolidated from migrations `001` through `006`. `report_subject_expense_reconciliation` and `person_expense_totals` are derived views and are therefore not shown as entities.

```mermaid
erDiagram

    disclosure_reports {
        BIGINT id PK
        TEXT domain
        INTEGER fiscal_year
        SMALLINT fiscal_quarter
        DATE period_start
        DATE period_end
        TEXT source_url
        TEXT travel_source_url
        TEXT hospitality_source_url
        TEXT contract_source_url
        TIMESTAMPTZ scraped_at
        TIMESTAMPTZ created_at
        TIMESTAMPTZ updated_at
    }

    people {
        BIGINT id PK
        TEXT first_name
        TEXT last_name
        TIMESTAMPTZ created_at
        TIMESTAMPTZ updated_at
    }

    mps {
        BIGINT id PK
        BIGINT person_id FK, UK
        TEXT province
        TEXT profile_url
        BYTEA avatar
        TEXT email
        TEXT website
        JSONB hill_office
        JSONB constituency_office
        JSONB roles
        TIMESTAMPTZ created_at
        TIMESTAMPTZ updated_at
    }

    mp_affiliations {
        BIGINT id PK
        BIGINT mp_id FK
        TEXT constituency
        TEXT caucus
        DATE effective_from
        DATE effective_to
        TIMESTAMPTZ created_at
        TIMESTAMPTZ updated_at
    }

    report_subjects {
        BIGINT id PK
        BIGINT report_id FK
        BIGINT person_id FK
        BIGINT mp_affiliation_id FK
        TEXT subject_kind
        TEXT caucus
        TEXT role
        NUMERIC_14_2 salaries
        NUMERIC_14_2 travel_total
        NUMERIC_14_2 hospitality_total
        NUMERIC_14_2 contract_total
        TEXT source_url
        TEXT travel_source_url
        TEXT hospitality_source_url
        TEXT contract_source_url
        TIMESTAMPTZ created_at
        TIMESTAMPTZ updated_at
    }

    mp_contract_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        TEXT supplier
        TEXT description
        TEXT category
        DATE expense_date
        TEXT source_date
        NUMERIC_14_2 total
        BOOLEAN is_refunded
        TIMESTAMPTZ created_at
    }

    mp_hospitality_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        DATE expense_date
        TEXT source_date
        TEXT location
        INTEGER attendance
        TEXT purpose
        TEXT event_type
        NUMERIC_14_2 total_cost
        NUMERIC_14_2 cost_per_person
        BOOLEAN is_refunded
        TIMESTAMPTZ created_at
    }

    mp_hospitality_expense_items {
        BIGINT id PK
        BIGINT hospitality_expense_id FK
        TEXT source_key
        TEXT claim
        TEXT supplier
        NUMERIC_14_2 cost
        TIMESTAMPTZ created_at
    }

    mp_travel_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT claim
        INTEGER sequence
        DATE period_start
        DATE period_end
        TEXT source_period_start
        TEXT source_period_end
        NUMERIC_14_2 transportation_cost
        NUMERIC_14_2 accommodation_cost
        NUMERIC_14_2 meals_and_incidentals_cost
        NUMERIC_14_2 total_cost
        DOUBLE regular_flight_points
        DOUBLE special_flight_points
        DOUBLE usa_flight_points
        BOOLEAN is_duplicate
        BOOLEAN is_refunded
        TIMESTAMPTZ created_at
    }

    mp_travelers {
        BIGINT id PK
        BIGINT travel_expense_id FK
        TEXT source_key
        BIGINT person_id FK
        TEXT traveller_type
        TEXT purpose
        DATE travel_date
        TEXT source_date
        TEXT departure_city
        TEXT destination_city
        TEXT transportation_mode
        DOUBLE distance
        TEXT distance_unit
        DOUBLE emissions
        TEXT emissions_unit
        TEXT travel_class
        TIMESTAMPTZ created_at
    }

    administration_contract_expenses {
        BIGINT id PK
        BIGINT report_id FK
        TEXT source_key
        TEXT description
        TEXT category
        DATE period_start
        DATE period_end
        TEXT source_period_start
        TEXT source_period_end
        NUMERIC_14_2 original_contract_value
        NUMERIC_14_2 amended_contract_value
        TIMESTAMPTZ created_at
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
        TEXT host
        TEXT purpose
        DATE expense_date
        TEXT source_date
        NUMERIC_14_2 total_cost
        NUMERIC_14_2 cost_per_person
        TEXT location
        TEXT attendance
        TIMESTAMPTZ created_at
    }

    administration_hospitality_suppliers {
        BIGINT id PK
        BIGINT hospitality_expense_id FK
        TEXT supplier
    }

    administration_travel_expenses {
        BIGINT id PK
        BIGINT report_id FK
        TEXT source_key
        TEXT event_id
        BIGINT traveller_person_id FK
        TEXT purpose
        DATE period_start
        DATE period_end
        TEXT source_period_start
        TEXT source_period_end
        NUMERIC_14_2 total_cost
        NUMERIC_14_2 transportation_cost
        NUMERIC_14_2 accommodations_cost
        NUMERIC_14_2 meals_and_incidentals_cost
        NUMERIC_14_2 other_cost
        TIMESTAMPTZ created_at
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
        TEXT supplier
        TEXT description
        TEXT category
        DATE expense_date
        TEXT source_date
        NUMERIC_14_2 total_cost
        TIMESTAMPTZ created_at
    }

    officer_hospitality_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        DATE expense_date
        TEXT source_date
        TEXT location
        INTEGER total_attendance
        TEXT purpose
        NUMERIC_14_2 total_cost
        NUMERIC_14_2 cost_per_person
        TIMESTAMPTZ created_at
    }

    officer_hospitality_suppliers {
        BIGINT id PK
        BIGINT hospitality_expense_id FK
        TEXT supplier
        NUMERIC_14_2 cost
    }

    officer_travel_expenses {
        BIGINT id PK
        BIGINT report_subject_id FK
        TEXT source_key
        DATE period_start
        DATE period_end
        TEXT source_period_start
        TEXT source_period_end
        TEXT departure_city
        NUMERIC_14_2 transportation_cost
        NUMERIC_14_2 accommodation_cost
        NUMERIC_14_2 meals_and_incidentals_cost
        NUMERIC_14_2 total_cost
        TEXT purpose
        BIGINT traveller_person_id FK
        TIMESTAMPTZ created_at
    }

    officer_travel_places {
        BIGINT id PK
        BIGINT travel_expense_id FK
        SMALLINT position
        TEXT place
    }

    people ||--o| mps : "has MP profile"
    mps ||--o{ mp_affiliations : "has affiliations"
    disclosure_reports ||--o{ report_subjects : "has subjects"
    people o|--o{ report_subjects : "is officer subject"
    mp_affiliations o|--o{ report_subjects : "is MP subject"

    report_subjects ||--o{ mp_contract_expenses : "has MP contracts"
    report_subjects ||--o{ mp_hospitality_expenses : "has MP hospitality"
    mp_hospitality_expenses ||--o{ mp_hospitality_expense_items : "has items"
    report_subjects ||--o{ mp_travel_expenses : "has MP travel"
    mp_travel_expenses ||--o{ mp_travelers : "has travellers"
    people o|--o{ mp_travelers : "identifies traveller"

    disclosure_reports ||--o{ administration_contract_expenses : "has admin contracts"
    administration_contract_expenses ||--o{ administration_contract_suppliers : "has suppliers"
    disclosure_reports ||--o{ administration_hospitality_expenses : "has admin hospitality"
    administration_hospitality_expenses ||--o{ administration_hospitality_suppliers : "has suppliers"
    disclosure_reports ||--o{ administration_travel_expenses : "has admin travel"
    people o|--o{ administration_travel_expenses : "identifies traveller"
    administration_travel_expenses ||--o{ administration_travel_places : "has places"

    report_subjects ||--o{ officer_contract_expenses : "has officer contracts"
    report_subjects ||--o{ officer_hospitality_expenses : "has officer hospitality"
    officer_hospitality_expenses ||--o{ officer_hospitality_suppliers : "has suppliers"
    report_subjects ||--o{ officer_travel_expenses : "has officer travel"
    people o|--o{ officer_travel_expenses : "identifies traveller"
    officer_travel_expenses ||--o{ officer_travel_places : "has places"



```

## Derived views

- `report_subject_expense_reconciliation` compares MP and officer summary totals with their loaded detail totals.
- `person_expense_totals` reports separately aggregated MP-subject, officer-subject, MP-traveller, officer-traveller, and Administration-traveller amounts.