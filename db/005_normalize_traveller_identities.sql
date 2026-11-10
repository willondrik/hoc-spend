-- Repairs confirmed traveller-only identity fragmentation and adds a
-- person-centric lookup view. Run once after migrations 002-004.
--
-- Generic labels such as "Designated Traveller" identify a traveller role, not
-- an individual. They must not remain linked to a people row. The named merges
-- below are intentionally explicit rather than heuristic because name order can
-- be ambiguous in historical source text.

BEGIN;

UPDATE mp_travelers AS traveller
SET person_id = NULL,
    traveller_type = 'Designated Traveller'
FROM people AS person
WHERE traveller.person_id = person.id
  AND lower(concat_ws(' ', person.first_name, person.last_name)) = 'designated traveller';

UPDATE officer_travel_expenses AS expense
SET traveller_person_id = NULL
FROM people AS person
WHERE expense.traveller_person_id = person.id
  AND lower(concat_ws(' ', person.first_name, person.last_name)) = 'designated traveller';

UPDATE administration_travel_expenses AS expense
SET traveller_person_id = NULL
FROM people AS person
WHERE expense.traveller_person_id = person.id
  AND lower(concat_ws(' ', person.first_name, person.last_name)) = 'designated traveller';

DELETE FROM people
WHERE lower(concat_ws(' ', first_name, last_name)) = 'designated traveller';

-- Verified source pairs: historical no-parentheses parsing stored the whole
-- display name in last_name, while the same disclosure period contains the
-- correctly split traveller identity.
WITH verified_names(full_name) AS (
    VALUES ('Kenny Chiu'), ('Michela Di Marco')
), pairs AS (
    SELECT malformed.id AS malformed_id, canonical.id AS canonical_id
    FROM verified_names
    JOIN people AS malformed
      ON malformed.first_name = ''
     AND lower(malformed.last_name) = lower(verified_names.full_name)
    JOIN people AS canonical
      ON lower(concat_ws(' ', canonical.first_name, canonical.last_name)) = lower(verified_names.full_name)
     AND canonical.first_name <> ''
)
UPDATE mp_travelers AS traveller
SET person_id = pairs.canonical_id
FROM pairs
WHERE traveller.person_id = pairs.malformed_id;

DELETE FROM people AS malformed
USING (
    SELECT malformed.id AS malformed_id
    FROM (VALUES ('Kenny Chiu'), ('Michela Di Marco')) AS verified_names(full_name)
    JOIN people AS malformed
      ON malformed.first_name = ''
     AND lower(malformed.last_name) = lower(verified_names.full_name)
    JOIN people AS canonical
      ON lower(concat_ws(' ', canonical.first_name, canonical.last_name)) = lower(verified_names.full_name)
     AND canonical.first_name <> ''
) AS pairs
WHERE malformed.id = pairs.malformed_id;

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
    WHERE subject_kind = 'officer'
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