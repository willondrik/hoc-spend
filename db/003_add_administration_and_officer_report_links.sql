-- Adds source links for category detail reports.
--
-- Administration category pages apply to an entire quarterly report, so their
-- links are stored on disclosure_reports. Officer category pages apply to an
-- individual officer summary, so their links are stored on report_subjects.
-- All fields are nullable because a category can have zero spending and no
-- House of Commons detail page. No data is backfilled by this migration.

BEGIN;

ALTER TABLE disclosure_reports ADD COLUMN IF NOT EXISTS travel_source_url TEXT;
ALTER TABLE disclosure_reports ADD COLUMN IF NOT EXISTS hospitality_source_url TEXT;
ALTER TABLE disclosure_reports ADD COLUMN IF NOT EXISTS contract_source_url TEXT;

ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS travel_source_url TEXT;
ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS hospitality_source_url TEXT;
ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS contract_source_url TEXT;

COMMIT;