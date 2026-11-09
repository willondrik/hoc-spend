-- Adds a queryable reconciliation view for MP and House Officer report
-- subjects. Run once against databases initialized before this view was added.
--
-- The three *_summary_total columns are the totals shown on the quarterly House
-- of Commons summary report. The three *_detail_total columns aggregate the
-- individually disclosed detail records. A non-zero delta or true mismatch flag
-- identifies a category that needs scraper investigation or a historical
-- re-scrape.

BEGIN;

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

COMMIT;