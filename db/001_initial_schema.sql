-- hoc-spend PostgreSQL fresh-install database and schema.
--
-- In a terminal, open psql as a database-creation-capable role:
--   sudo -u postgres psql -d postgres
--
-- At the resulting `postgres=#` prompt, paste this entire file. This creates
-- the hoc_spend database, switches to it, and creates all tables and indexes.
--
-- Run this only when hoc_spend does not already exist.

CREATE DATABASE hoc_spend;

\c hoc_spend

BEGIN;

CREATE TABLE IF NOT EXISTS disclosure_reports (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    domain TEXT NOT NULL CHECK (domain IN ('mp', 'administration', 'officer')),
    fiscal_year INTEGER NOT NULL CHECK (fiscal_year BETWEEN 1867 AND 9999),
    fiscal_quarter SMALLINT NOT NULL CHECK (fiscal_quarter BETWEEN 1 AND 4),
    period_start DATE,
    period_end DATE,
    source_url TEXT NOT NULL,
    travel_source_url TEXT,
    hospitality_source_url TEXT,
    contract_source_url TEXT,
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT disclosure_reports_period_order CHECK (
        period_start IS NULL OR period_end IS NULL OR period_start <= period_end
    ),
    CONSTRAINT disclosure_reports_domain_period_key UNIQUE (domain, fiscal_year, fiscal_quarter),
    CONSTRAINT disclosure_reports_source_url_key UNIQUE (source_url)
);

-- A normalized person record is used when a source value identifies an MP,
-- officer, traveller, or another named participant. Source pages do not expose
-- durable person IDs, so name fields remain the source-of-truth identifiers.
-- MP-specific profile data belongs to mps, not to generic traveller records.
CREATE TABLE IF NOT EXISTS people (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- A durable MP entity owns profile enrichment independently of any quarterly
-- disclosure report. Travellers remain linked only to people.
CREATE TABLE IF NOT EXISTS mps (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    person_id BIGINT NOT NULL UNIQUE REFERENCES people(id) ON DELETE CASCADE,
    province TEXT,
    profile_url TEXT,
    avatar BYTEA,
    email TEXT,
    website TEXT,
    hill_office JSONB,
    constituency_office JSONB,
    roles JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mps_profile_url_key UNIQUE (profile_url)
);

-- An MP affiliation records the caucus and riding associated with one or more
-- quarterly disclosures. It preserves historic spending context while mps
-- remains the durable identity and owner of the current profile.
CREATE TABLE IF NOT EXISTS mp_affiliations (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    mp_id BIGINT NOT NULL REFERENCES mps(id) ON DELETE RESTRICT,
    constituency TEXT NOT NULL DEFAULT '',
    caucus TEXT NOT NULL DEFAULT '',
    effective_from DATE,
    effective_to DATE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mp_affiliations_period_order CHECK (
        effective_from IS NULL OR effective_to IS NULL OR effective_from <= effective_to
    ),
    CONSTRAINT mp_affiliations_mp_constituency_caucus_key UNIQUE (mp_id, constituency, caucus)
);

-- A report subject is the person/office whose quarterly summary is disclosed.
-- Administration detail reports currently have no parsed subject summary, so
-- their detail entries attach directly to disclosure_reports instead.
CREATE TABLE IF NOT EXISTS report_subjects (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES disclosure_reports(id) ON DELETE CASCADE,
    person_id BIGINT REFERENCES people(id) ON DELETE RESTRICT,
    mp_affiliation_id BIGINT REFERENCES mp_affiliations(id) ON DELETE RESTRICT,
    subject_kind TEXT NOT NULL CHECK (subject_kind IN ('mp', 'officer')),
    caucus TEXT,
    role TEXT,
    salaries NUMERIC(14, 2) NOT NULL DEFAULT 0,
    travel_total NUMERIC(14, 2) NOT NULL DEFAULT 0,
    hospitality_total NUMERIC(14, 2) NOT NULL DEFAULT 0,
    contract_total NUMERIC(14, 2) NOT NULL DEFAULT 0,
    source_url TEXT,
    travel_source_url TEXT,
    hospitality_source_url TEXT,
    contract_source_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT report_subjects_subject_reference_check CHECK (
        (subject_kind = 'mp' AND mp_affiliation_id IS NOT NULL AND person_id IS NULL)
        OR (subject_kind = 'officer' AND person_id IS NOT NULL AND mp_affiliation_id IS NULL)
    ),
    CONSTRAINT report_subjects_report_person_key UNIQUE (report_id, person_id),
    CONSTRAINT report_subjects_report_mp_affiliation_key UNIQUE (report_id, mp_affiliation_id)
);

CREATE TABLE IF NOT EXISTS mp_contract_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_subject_id BIGINT NOT NULL REFERENCES report_subjects(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    supplier TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    expense_date DATE,
    source_date TEXT NOT NULL DEFAULT '',
    total NUMERIC(14, 2) NOT NULL,
    is_refunded BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mp_contract_expenses_subject_source_key UNIQUE (report_subject_id, source_key)
);

CREATE TABLE IF NOT EXISTS mp_hospitality_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_subject_id BIGINT NOT NULL REFERENCES report_subjects(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    expense_date DATE,
    source_date TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    attendance INTEGER NOT NULL DEFAULT 0 CHECK (attendance >= 0),
    purpose TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    total_cost NUMERIC(14, 2) NOT NULL,
    cost_per_person NUMERIC(14, 2) NOT NULL DEFAULT 0,
    is_refunded BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mp_hospitality_expenses_subject_source_key UNIQUE (report_subject_id, source_key)
);

CREATE TABLE IF NOT EXISTS mp_hospitality_expense_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    hospitality_expense_id BIGINT NOT NULL REFERENCES mp_hospitality_expenses(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    claim TEXT NOT NULL DEFAULT '',
    supplier TEXT NOT NULL,
    cost NUMERIC(14, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mp_hospitality_expense_items_expense_source_key UNIQUE (hospitality_expense_id, source_key)
);

CREATE TABLE IF NOT EXISTS mp_travel_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_subject_id BIGINT NOT NULL REFERENCES report_subjects(id) ON DELETE CASCADE,
    claim TEXT NOT NULL DEFAULT '',
    sequence INTEGER NOT NULL DEFAULT 0 CHECK (sequence >= 0),
    period_start DATE,
    period_end DATE,
    source_period_start TEXT NOT NULL DEFAULT '',
    source_period_end TEXT NOT NULL DEFAULT '',
    transportation_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    accommodation_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    meals_and_incidentals_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    total_cost NUMERIC(14, 2) NOT NULL,
    regular_flight_points DOUBLE PRECISION NOT NULL DEFAULT 0,
    special_flight_points DOUBLE PRECISION NOT NULL DEFAULT 0,
    usa_flight_points DOUBLE PRECISION NOT NULL DEFAULT 0,
    is_duplicate BOOLEAN NOT NULL DEFAULT FALSE,
    is_refunded BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mp_travel_expenses_period_order CHECK (
        period_start IS NULL OR period_end IS NULL OR period_start <= period_end
    ),
    CONSTRAINT mp_travel_expenses_subject_sequence_key UNIQUE (report_subject_id, sequence)
);

CREATE TABLE IF NOT EXISTS mp_travelers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    travel_expense_id BIGINT NOT NULL REFERENCES mp_travel_expenses(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    person_id BIGINT REFERENCES people(id) ON DELETE SET NULL,
    traveller_type TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL DEFAULT '',
    travel_date DATE,
    source_date TEXT NOT NULL DEFAULT '',
    departure_city TEXT NOT NULL DEFAULT '',
    destination_city TEXT NOT NULL DEFAULT '',
    transportation_mode TEXT NOT NULL DEFAULT '',
    distance DOUBLE PRECISION,
    distance_unit TEXT,
    emissions DOUBLE PRECISION,
    emissions_unit TEXT,
    travel_class TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT mp_travelers_expense_source_key UNIQUE (travel_expense_id, source_key)
);

CREATE TABLE IF NOT EXISTS administration_contract_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES disclosure_reports(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    period_start DATE,
    period_end DATE,
    source_period_start TEXT NOT NULL DEFAULT '',
    source_period_end TEXT NOT NULL DEFAULT '',
    original_contract_value NUMERIC(14, 2) NOT NULL,
    amended_contract_value NUMERIC(14, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT administration_contract_expenses_period_order CHECK (
        period_start IS NULL OR period_end IS NULL OR period_start <= period_end
    ),
    CONSTRAINT administration_contract_expenses_report_source_key UNIQUE (report_id, source_key)
);

CREATE TABLE IF NOT EXISTS administration_contract_suppliers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contract_expense_id BIGINT NOT NULL REFERENCES administration_contract_expenses(id) ON DELETE CASCADE,
    supplier TEXT NOT NULL,
    CONSTRAINT administration_contract_suppliers_key UNIQUE (contract_expense_id, supplier)
);

CREATE TABLE IF NOT EXISTS administration_hospitality_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES disclosure_reports(id) ON DELETE CASCADE,
    event_id TEXT NOT NULL,
    host TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL DEFAULT '',
    expense_date DATE,
    source_date TEXT NOT NULL DEFAULT '',
    total_cost NUMERIC(14, 2) NOT NULL,
    cost_per_person NUMERIC(14, 2) NOT NULL DEFAULT 0,
    location TEXT NOT NULL DEFAULT '',
    attendance TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT administration_hospitality_expenses_report_event_key UNIQUE (report_id, event_id)
);

CREATE TABLE IF NOT EXISTS administration_hospitality_suppliers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    hospitality_expense_id BIGINT NOT NULL REFERENCES administration_hospitality_expenses(id) ON DELETE CASCADE,
    supplier TEXT NOT NULL,
    CONSTRAINT administration_hospitality_suppliers_key UNIQUE (hospitality_expense_id, supplier)
);

CREATE TABLE IF NOT EXISTS administration_travel_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_id BIGINT NOT NULL REFERENCES disclosure_reports(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    event_id TEXT NOT NULL,
    traveller_person_id BIGINT REFERENCES people(id) ON DELETE SET NULL,
    purpose TEXT NOT NULL DEFAULT '',
    period_start DATE,
    period_end DATE,
    source_period_start TEXT NOT NULL DEFAULT '',
    source_period_end TEXT NOT NULL DEFAULT '',
    total_cost NUMERIC(14, 2) NOT NULL,
    transportation_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    accommodations_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    meals_and_incidentals_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    other_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT administration_travel_expenses_period_order CHECK (
        period_start IS NULL OR period_end IS NULL OR period_start <= period_end
    ),
    CONSTRAINT administration_travel_expenses_report_source_key UNIQUE (report_id, source_key)
);

CREATE TABLE IF NOT EXISTS administration_travel_places (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    travel_expense_id BIGINT NOT NULL REFERENCES administration_travel_expenses(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL CHECK (position >= 0),
    place TEXT NOT NULL,
    CONSTRAINT administration_travel_places_key UNIQUE (travel_expense_id, position)
);

CREATE TABLE IF NOT EXISTS officer_contract_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_subject_id BIGINT NOT NULL REFERENCES report_subjects(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    supplier TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    expense_date DATE,
    source_date TEXT NOT NULL DEFAULT '',
    total_cost NUMERIC(14, 2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT officer_contract_expenses_subject_source_key UNIQUE (report_subject_id, source_key)
);

CREATE TABLE IF NOT EXISTS officer_hospitality_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_subject_id BIGINT NOT NULL REFERENCES report_subjects(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    expense_date DATE,
    source_date TEXT NOT NULL DEFAULT '',
    location TEXT NOT NULL DEFAULT '',
    total_attendance INTEGER NOT NULL DEFAULT 0 CHECK (total_attendance >= 0),
    purpose TEXT NOT NULL DEFAULT '',
    total_cost NUMERIC(14, 2) NOT NULL,
    cost_per_person NUMERIC(14, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT officer_hospitality_expenses_subject_source_key UNIQUE (report_subject_id, source_key)
);

CREATE TABLE IF NOT EXISTS officer_hospitality_suppliers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    hospitality_expense_id BIGINT NOT NULL REFERENCES officer_hospitality_expenses(id) ON DELETE CASCADE,
    supplier TEXT NOT NULL,
    cost NUMERIC(14, 2) NOT NULL,
    CONSTRAINT officer_hospitality_suppliers_key UNIQUE (hospitality_expense_id, supplier, cost)
);

CREATE TABLE IF NOT EXISTS officer_travel_expenses (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    report_subject_id BIGINT NOT NULL REFERENCES report_subjects(id) ON DELETE CASCADE,
    source_key TEXT NOT NULL,
    period_start DATE,
    period_end DATE,
    source_period_start TEXT NOT NULL DEFAULT '',
    source_period_end TEXT NOT NULL DEFAULT '',
    departure_city TEXT NOT NULL DEFAULT '',
    transportation_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    accommodation_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    meals_and_incidentals_cost NUMERIC(14, 2) NOT NULL DEFAULT 0,
    total_cost NUMERIC(14, 2) NOT NULL,
    purpose TEXT NOT NULL DEFAULT '',
    traveller_person_id BIGINT REFERENCES people(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT officer_travel_expenses_period_order CHECK (
        period_start IS NULL OR period_end IS NULL OR period_start <= period_end
    ),
    CONSTRAINT officer_travel_expenses_subject_source_key UNIQUE (report_subject_id, source_key)
);

CREATE TABLE IF NOT EXISTS officer_travel_places (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    travel_expense_id BIGINT NOT NULL REFERENCES officer_travel_expenses(id) ON DELETE CASCADE,
    position SMALLINT NOT NULL CHECK (position >= 0),
    place TEXT NOT NULL,
    CONSTRAINT officer_travel_places_key UNIQUE (travel_expense_id, position)
);

CREATE INDEX IF NOT EXISTS disclosure_reports_domain_period_idx
    ON disclosure_reports (domain, fiscal_year DESC, fiscal_quarter DESC);
CREATE INDEX IF NOT EXISTS mp_affiliations_mp_idx ON mp_affiliations (mp_id);
CREATE INDEX IF NOT EXISTS report_subjects_report_idx ON report_subjects (report_id);
CREATE INDEX IF NOT EXISTS report_subjects_person_idx ON report_subjects (person_id);
CREATE INDEX IF NOT EXISTS report_subjects_mp_affiliation_idx ON report_subjects (mp_affiliation_id);
CREATE INDEX IF NOT EXISTS mp_contract_expenses_subject_date_idx
    ON mp_contract_expenses (report_subject_id, expense_date DESC);
CREATE INDEX IF NOT EXISTS mp_contract_expenses_supplier_idx ON mp_contract_expenses (supplier);
CREATE INDEX IF NOT EXISTS mp_hospitality_expenses_subject_date_idx
    ON mp_hospitality_expenses (report_subject_id, expense_date DESC);
CREATE INDEX IF NOT EXISTS mp_travel_expenses_subject_date_idx
    ON mp_travel_expenses (report_subject_id, period_start DESC);
CREATE INDEX IF NOT EXISTS administration_contract_expenses_report_idx
    ON administration_contract_expenses (report_id);
CREATE INDEX IF NOT EXISTS administration_hospitality_expenses_report_date_idx
    ON administration_hospitality_expenses (report_id, expense_date DESC);
CREATE INDEX IF NOT EXISTS administration_travel_expenses_report_period_idx
    ON administration_travel_expenses (report_id, period_start DESC);
CREATE INDEX IF NOT EXISTS officer_contract_expenses_subject_date_idx
    ON officer_contract_expenses (report_subject_id, expense_date DESC);
CREATE INDEX IF NOT EXISTS officer_hospitality_expenses_subject_date_idx
    ON officer_hospitality_expenses (report_subject_id, expense_date DESC);
CREATE INDEX IF NOT EXISTS officer_travel_expenses_subject_period_idx
    ON officer_travel_expenses (report_subject_id, period_start DESC);

-- Compares the quarterly report totals with sums of individually disclosed
-- detail rows. It is intentionally a view, so it always reflects a re-scrape
-- without maintaining duplicate aggregate data.
CREATE OR REPLACE VIEW report_subject_expense_reconciliation AS
WITH contract_totals AS (
    SELECT report_subject_id, SUM(total) AS total
    FROM mp_contract_expenses
    GROUP BY report_subject_id
    UNION ALL
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM officer_contract_expenses
    GROUP BY report_subject_id
),
hospitality_totals AS (
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM mp_hospitality_expenses
    GROUP BY report_subject_id
    UNION ALL
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM officer_hospitality_expenses
    GROUP BY report_subject_id
),
travel_totals AS (
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM mp_travel_expenses
    GROUP BY report_subject_id
    UNION ALL
    SELECT report_subject_id, SUM(total_cost) AS total
    FROM officer_travel_expenses
    GROUP BY report_subject_id
)
SELECT
    disclosure_reports.domain,
    disclosure_reports.fiscal_year,
    disclosure_reports.fiscal_quarter,
    report_subjects.id AS report_subject_id,
    report_subjects.subject_kind,
    COALESCE(mp_people.first_name, officer_people.first_name, '') AS first_name,
    COALESCE(mp_people.last_name, officer_people.last_name, '') AS last_name,
    mp_affiliations.constituency,
    report_subjects.role,
    report_subjects.contract_total AS contract_summary_total,
    COALESCE(contract_totals.total, 0)::NUMERIC(14, 2) AS contract_detail_total,
    (COALESCE(contract_totals.total, 0) - report_subjects.contract_total)::NUMERIC(14, 2) AS contract_delta,
    report_subjects.hospitality_total AS hospitality_summary_total,
    COALESCE(hospitality_totals.total, 0)::NUMERIC(14, 2) AS hospitality_detail_total,
    (COALESCE(hospitality_totals.total, 0) - report_subjects.hospitality_total)::NUMERIC(14, 2) AS hospitality_delta,
    report_subjects.travel_total AS travel_summary_total,
    COALESCE(travel_totals.total, 0)::NUMERIC(14, 2) AS travel_detail_total,
    (COALESCE(travel_totals.total, 0) - report_subjects.travel_total)::NUMERIC(14, 2) AS travel_delta,
    report_subjects.contract_total <> COALESCE(contract_totals.total, 0) AS contract_mismatch,
    report_subjects.hospitality_total <> COALESCE(hospitality_totals.total, 0) AS hospitality_mismatch,
    report_subjects.travel_total <> COALESCE(travel_totals.total, 0) AS travel_mismatch
FROM report_subjects
JOIN disclosure_reports ON disclosure_reports.id = report_subjects.report_id
LEFT JOIN mp_affiliations ON mp_affiliations.id = report_subjects.mp_affiliation_id
LEFT JOIN mps ON mps.id = mp_affiliations.mp_id
LEFT JOIN people AS mp_people ON mp_people.id = mps.person_id
LEFT JOIN people AS officer_people ON officer_people.id = report_subjects.person_id
LEFT JOIN contract_totals ON contract_totals.report_subject_id = report_subjects.id
LEFT JOIN hospitality_totals ON hospitality_totals.report_subject_id = report_subjects.id
LEFT JOIN travel_totals ON travel_totals.report_subject_id = report_subjects.id
WHERE disclosure_reports.domain IN ('mp', 'officer');

-- Person-centric lookup totals. Subject totals and traveller totals are kept
-- separate because a person's own travel may also be included in their MP or
-- officer subject disclosure; summing all roles together would double-count.
CREATE OR REPLACE VIEW person_expense_totals AS
WITH person_amounts AS (
    SELECT mps.person_id, 'mp_subject'::TEXT AS role, report_subjects.travel_total + report_subjects.hospitality_total + report_subjects.contract_total AS amount
    FROM report_subjects
    JOIN mp_affiliations ON mp_affiliations.id = report_subjects.mp_affiliation_id
    JOIN mps ON mps.id = mp_affiliations.mp_id
    WHERE report_subjects.subject_kind = 'mp'

    UNION ALL

    SELECT report_subjects.person_id, 'officer_subject'::TEXT, report_subjects.travel_total + report_subjects.hospitality_total + report_subjects.contract_total
    FROM report_subjects
    WHERE report_subjects.subject_kind = 'officer'
      AND report_subjects.person_id IS NOT NULL

    UNION ALL

    SELECT mp_travelers.person_id, 'mp_traveller'::TEXT, mp_travel_expenses.total_cost
    FROM mp_travelers
    JOIN mp_travel_expenses ON mp_travel_expenses.id = mp_travelers.travel_expense_id
    WHERE mp_travelers.person_id IS NOT NULL

    UNION ALL

    SELECT officer_travel_expenses.traveller_person_id, 'officer_traveller'::TEXT, officer_travel_expenses.total_cost
    FROM officer_travel_expenses
    WHERE officer_travel_expenses.traveller_person_id IS NOT NULL

    UNION ALL

    SELECT administration_travel_expenses.traveller_person_id, 'administration_traveller'::TEXT, administration_travel_expenses.total_cost
    FROM administration_travel_expenses
    WHERE administration_travel_expenses.traveller_person_id IS NOT NULL
)
SELECT
    people.id AS person_id,
    people.first_name,
    people.last_name,
    concat_ws(' ', people.first_name, people.last_name) AS full_name,
    COALESCE(SUM(amount) FILTER (WHERE role = 'mp_subject'), 0)::NUMERIC(14, 2) AS mp_subject_total,
    COALESCE(SUM(amount) FILTER (WHERE role = 'officer_subject'), 0)::NUMERIC(14, 2) AS officer_subject_total,
    COALESCE(SUM(amount) FILTER (WHERE role = 'mp_traveller'), 0)::NUMERIC(14, 2) AS mp_traveller_total,
    COALESCE(SUM(amount) FILTER (WHERE role = 'officer_traveller'), 0)::NUMERIC(14, 2) AS officer_traveller_total,
    COALESCE(SUM(amount) FILTER (WHERE role = 'administration_traveller'), 0)::NUMERIC(14, 2) AS administration_traveller_total
FROM people
LEFT JOIN person_amounts ON person_amounts.person_id = people.id
GROUP BY people.id, people.first_name, people.last_name;

COMMIT;