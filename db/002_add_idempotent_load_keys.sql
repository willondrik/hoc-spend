-- Adds deterministic source identities required by the incremental detail
-- loaders. Run once against a database created before source_key columns were
-- added to 001_initial_schema.sql.
--
-- Existing rows receive a legacy key based on their primary key. This preserves
-- their data and makes new loads idempotent; it does not attempt to infer a
-- source identity for rows imported before this migration.

BEGIN;

ALTER TABLE mp_contract_expenses ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE mp_contract_expenses SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE mp_contract_expenses ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE mp_contract_expenses DROP CONSTRAINT IF EXISTS mp_contract_expenses_subject_source_key;
ALTER TABLE mp_contract_expenses ADD CONSTRAINT mp_contract_expenses_subject_source_key UNIQUE (report_subject_id, source_key);

ALTER TABLE mp_hospitality_expenses ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE mp_hospitality_expenses SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE mp_hospitality_expenses ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE mp_hospitality_expenses DROP CONSTRAINT IF EXISTS mp_hospitality_expenses_subject_source_key;
ALTER TABLE mp_hospitality_expenses ADD CONSTRAINT mp_hospitality_expenses_subject_source_key UNIQUE (report_subject_id, source_key);

ALTER TABLE mp_hospitality_expense_items ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE mp_hospitality_expense_items SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE mp_hospitality_expense_items ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE mp_hospitality_expense_items DROP CONSTRAINT IF EXISTS mp_hospitality_expense_items_expense_source_key;
ALTER TABLE mp_hospitality_expense_items ADD CONSTRAINT mp_hospitality_expense_items_expense_source_key UNIQUE (hospitality_expense_id, source_key);

ALTER TABLE mp_travelers ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE mp_travelers SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE mp_travelers ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE mp_travelers DROP CONSTRAINT IF EXISTS mp_travelers_expense_source_key;
ALTER TABLE mp_travelers ADD CONSTRAINT mp_travelers_expense_source_key UNIQUE (travel_expense_id, source_key);

ALTER TABLE administration_contract_expenses ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE administration_contract_expenses SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE administration_contract_expenses ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE administration_contract_expenses DROP CONSTRAINT IF EXISTS administration_contract_expenses_report_source_key;
ALTER TABLE administration_contract_expenses ADD CONSTRAINT administration_contract_expenses_report_source_key UNIQUE (report_id, source_key);

ALTER TABLE officer_contract_expenses ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE officer_contract_expenses SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE officer_contract_expenses ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE officer_contract_expenses DROP CONSTRAINT IF EXISTS officer_contract_expenses_subject_source_key;
ALTER TABLE officer_contract_expenses ADD CONSTRAINT officer_contract_expenses_subject_source_key UNIQUE (report_subject_id, source_key);

ALTER TABLE officer_hospitality_expenses ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE officer_hospitality_expenses SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE officer_hospitality_expenses ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE officer_hospitality_expenses DROP CONSTRAINT IF EXISTS officer_hospitality_expenses_subject_source_key;
ALTER TABLE officer_hospitality_expenses ADD CONSTRAINT officer_hospitality_expenses_subject_source_key UNIQUE (report_subject_id, source_key);

ALTER TABLE officer_travel_expenses ADD COLUMN IF NOT EXISTS source_key TEXT;
UPDATE officer_travel_expenses SET source_key = 'legacy:' || id WHERE source_key IS NULL;
ALTER TABLE officer_travel_expenses ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE officer_travel_expenses DROP CONSTRAINT IF EXISTS officer_travel_expenses_subject_source_key;
ALTER TABLE officer_travel_expenses ADD CONSTRAINT officer_travel_expenses_subject_source_key UNIQUE (report_subject_id, source_key);

COMMIT;