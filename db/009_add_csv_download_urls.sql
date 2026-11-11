-- Stores the House of Commons CSV-export URLs for each report category.
--
-- MPs and officers own category pages per report subject, while Administration
-- category pages apply to an entire quarterly disclosure report. Existing rows
-- remain NULL until their category pages are re-scraped.

BEGIN;

ALTER TABLE disclosure_reports ADD COLUMN IF NOT EXISTS travel_csv_url TEXT;
ALTER TABLE disclosure_reports ADD COLUMN IF NOT EXISTS hospitality_csv_url TEXT;
ALTER TABLE disclosure_reports ADD COLUMN IF NOT EXISTS contract_csv_url TEXT;

ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS travel_csv_url TEXT;
ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS hospitality_csv_url TEXT;
ALTER TABLE report_subjects ADD COLUMN IF NOT EXISTS contract_csv_url TEXT;

COMMIT;