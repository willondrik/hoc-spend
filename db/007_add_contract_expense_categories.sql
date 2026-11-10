-- Adds broad, query-friendly categories to contract expense rows while
-- preserving each House of Commons description as the source-of-truth text.
--
-- The category taxonomy is maintained by the Go detail loaders. This migration
-- applies the same initial deterministic rules to existing rows. Descriptions
-- that do not clearly match a category are retained as "other".

BEGIN;

ALTER TABLE mp_contract_expenses
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'other';
ALTER TABLE administration_contract_expenses
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'other';
ALTER TABLE officer_contract_expenses
    ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT 'other';

WITH categorized AS (
    SELECT
        id,
        CASE
            WHEN lower(description) LIKE ANY (ARRAY['%print%', '%translation%', '%translate%', '%interpretation%', '%interpreter%', '%transcription%']) THEN 'printing_and_translation'
            WHEN lower(description) LIKE ANY (ARRAY['%software%', '%licence%', '%license%', '%cloud%', '%hosting%', '%website%', '%web site%', '%web development%', '%computer%', '%laptop%', '%tablet%', '%cybersecurity%', '%cyber security%', '%it support%', '%information technology%']) THEN 'information_technology'
            WHEN lower(description) LIKE ANY (ARRAY['%airfare%', '%air fare%', '%flight%', '%hotel%', '%accommodation%', '%transportation%', '%travel%']) THEN 'travel'
            WHEN lower(description) LIKE ANY (ARRAY['%training%', '%conference%', '%seminar%', '%workshop%', '%registration%', '%event%']) THEN 'training_and_events'
            WHEN lower(description) LIKE ANY (ARRAY['%advertising%', '%media%', '%communications%', '%social media%', '%newsletter%', '%photography%', '%videography%', '%video production%', '%graphic design%']) THEN 'communications'
            WHEN lower(description) LIKE ANY (ARRAY['%consult%', '%legal%', '%lawyer%', '%accounting%', '%audit%', '%research%', '%recruit%', '%advisory%']) THEN 'professional_services'
            WHEN lower(description) LIKE ANY (ARRAY['%rent%', '%lease%', '%cleaning%', '%security%', '%repair%', '%maintenance%', '%utilities%', '%facility%', '%facilities%']) THEN 'facilities'
            WHEN lower(description) LIKE ANY (ARRAY['%office supplies%', '%stationery%', '%furniture%', '%equipment%', '%courier%', '%postage%']) THEN 'office_and_equipment'
            ELSE 'other'
        END AS category
    FROM mp_contract_expenses
)
UPDATE mp_contract_expenses
SET category = categorized.category
FROM categorized
WHERE mp_contract_expenses.id = categorized.id;

WITH categorized AS (
    SELECT
        id,
        CASE
            WHEN lower(description) LIKE ANY (ARRAY['%print%', '%translation%', '%translate%', '%interpretation%', '%interpreter%', '%transcription%']) THEN 'printing_and_translation'
            WHEN lower(description) LIKE ANY (ARRAY['%software%', '%licence%', '%license%', '%cloud%', '%hosting%', '%website%', '%web site%', '%web development%', '%computer%', '%laptop%', '%tablet%', '%cybersecurity%', '%cyber security%', '%it support%', '%information technology%']) THEN 'information_technology'
            WHEN lower(description) LIKE ANY (ARRAY['%airfare%', '%air fare%', '%flight%', '%hotel%', '%accommodation%', '%transportation%', '%travel%']) THEN 'travel'
            WHEN lower(description) LIKE ANY (ARRAY['%training%', '%conference%', '%seminar%', '%workshop%', '%registration%', '%event%']) THEN 'training_and_events'
            WHEN lower(description) LIKE ANY (ARRAY['%advertising%', '%media%', '%communications%', '%social media%', '%newsletter%', '%photography%', '%videography%', '%video production%', '%graphic design%']) THEN 'communications'
            WHEN lower(description) LIKE ANY (ARRAY['%consult%', '%legal%', '%lawyer%', '%accounting%', '%audit%', '%research%', '%recruit%', '%advisory%']) THEN 'professional_services'
            WHEN lower(description) LIKE ANY (ARRAY['%rent%', '%lease%', '%cleaning%', '%security%', '%repair%', '%maintenance%', '%utilities%', '%facility%', '%facilities%']) THEN 'facilities'
            WHEN lower(description) LIKE ANY (ARRAY['%office supplies%', '%stationery%', '%furniture%', '%equipment%', '%courier%', '%postage%']) THEN 'office_and_equipment'
            ELSE 'other'
        END AS category
    FROM administration_contract_expenses
)
UPDATE administration_contract_expenses
SET category = categorized.category
FROM categorized
WHERE administration_contract_expenses.id = categorized.id;

WITH categorized AS (
    SELECT
        id,
        CASE
            WHEN lower(description) LIKE ANY (ARRAY['%print%', '%translation%', '%translate%', '%interpretation%', '%interpreter%', '%transcription%']) THEN 'printing_and_translation'
            WHEN lower(description) LIKE ANY (ARRAY['%software%', '%licence%', '%license%', '%cloud%', '%hosting%', '%website%', '%web site%', '%web development%', '%computer%', '%laptop%', '%tablet%', '%cybersecurity%', '%cyber security%', '%it support%', '%information technology%']) THEN 'information_technology'
            WHEN lower(description) LIKE ANY (ARRAY['%airfare%', '%air fare%', '%flight%', '%hotel%', '%accommodation%', '%transportation%', '%travel%']) THEN 'travel'
            WHEN lower(description) LIKE ANY (ARRAY['%training%', '%conference%', '%seminar%', '%workshop%', '%registration%', '%event%']) THEN 'training_and_events'
            WHEN lower(description) LIKE ANY (ARRAY['%advertising%', '%media%', '%communications%', '%social media%', '%newsletter%', '%photography%', '%videography%', '%video production%', '%graphic design%']) THEN 'communications'
            WHEN lower(description) LIKE ANY (ARRAY['%consult%', '%legal%', '%lawyer%', '%accounting%', '%audit%', '%research%', '%recruit%', '%advisory%']) THEN 'professional_services'
            WHEN lower(description) LIKE ANY (ARRAY['%rent%', '%lease%', '%cleaning%', '%security%', '%repair%', '%maintenance%', '%utilities%', '%facility%', '%facilities%']) THEN 'facilities'
            WHEN lower(description) LIKE ANY (ARRAY['%office supplies%', '%stationery%', '%furniture%', '%equipment%', '%courier%', '%postage%']) THEN 'office_and_equipment'
            ELSE 'other'
        END AS category
    FROM officer_contract_expenses
)
UPDATE officer_contract_expenses
SET category = categorized.category
FROM categorized
WHERE officer_contract_expenses.id = categorized.id;

ALTER TABLE mp_contract_expenses
    DROP CONSTRAINT IF EXISTS mp_contract_expenses_category_check,
    ADD CONSTRAINT mp_contract_expenses_category_check CHECK (category IN (
        'information_technology', 'communications', 'professional_services',
        'office_and_equipment', 'facilities', 'printing_and_translation',
        'training_and_events', 'travel', 'other'
    ));
ALTER TABLE administration_contract_expenses
    DROP CONSTRAINT IF EXISTS administration_contract_expenses_category_check,
    ADD CONSTRAINT administration_contract_expenses_category_check CHECK (category IN (
        'information_technology', 'communications', 'professional_services',
        'office_and_equipment', 'facilities', 'printing_and_translation',
        'training_and_events', 'travel', 'other'
    ));
ALTER TABLE officer_contract_expenses
    DROP CONSTRAINT IF EXISTS officer_contract_expenses_category_check,
    ADD CONSTRAINT officer_contract_expenses_category_check CHECK (category IN (
        'information_technology', 'communications', 'professional_services',
        'office_and_equipment', 'facilities', 'printing_and_translation',
        'training_and_events', 'travel', 'other'
    ));

CREATE INDEX IF NOT EXISTS mp_contract_expenses_category_idx
    ON mp_contract_expenses (category);
CREATE INDEX IF NOT EXISTS administration_contract_expenses_category_idx
    ON administration_contract_expenses (category);
CREATE INDEX IF NOT EXISTS officer_contract_expenses_category_idx
    ON officer_contract_expenses (category);

COMMIT;