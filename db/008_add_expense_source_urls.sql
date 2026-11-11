-- Stores the House of Commons page containing every top-level expense row.
--
-- Category detail URLs are preferred. Older records can lack those links, so
-- their owning subject or quarterly report URL is retained as a usable source
-- fallback before the new columns are made mandatory.

BEGIN;

ALTER TABLE mp_contract_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE mp_hospitality_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE mp_travel_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE administration_contract_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE administration_hospitality_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE administration_travel_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE officer_contract_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE officer_hospitality_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;
ALTER TABLE officer_travel_expenses ADD COLUMN IF NOT EXISTS source_url TEXT;

UPDATE mp_contract_expenses AS expense
SET source_url = COALESCE(NULLIF(subject.contract_source_url, ''), NULLIF(subject.source_url, ''), report.source_url)
FROM report_subjects AS subject
JOIN disclosure_reports AS report ON report.id = subject.report_id
WHERE expense.report_subject_id = subject.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE mp_hospitality_expenses AS expense
SET source_url = COALESCE(NULLIF(subject.hospitality_source_url, ''), NULLIF(subject.source_url, ''), report.source_url)
FROM report_subjects AS subject
JOIN disclosure_reports AS report ON report.id = subject.report_id
WHERE expense.report_subject_id = subject.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE mp_travel_expenses AS expense
SET source_url = COALESCE(NULLIF(subject.travel_source_url, ''), NULLIF(subject.source_url, ''), report.source_url)
FROM report_subjects AS subject
JOIN disclosure_reports AS report ON report.id = subject.report_id
WHERE expense.report_subject_id = subject.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE administration_contract_expenses AS expense
SET source_url = COALESCE(NULLIF(report.contract_source_url, ''), report.source_url)
FROM disclosure_reports AS report
WHERE expense.report_id = report.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE administration_hospitality_expenses AS expense
SET source_url = COALESCE(NULLIF(report.hospitality_source_url, ''), report.source_url)
FROM disclosure_reports AS report
WHERE expense.report_id = report.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE administration_travel_expenses AS expense
SET source_url = COALESCE(NULLIF(report.travel_source_url, ''), report.source_url)
FROM disclosure_reports AS report
WHERE expense.report_id = report.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE officer_contract_expenses AS expense
SET source_url = COALESCE(NULLIF(subject.contract_source_url, ''), NULLIF(subject.source_url, ''), report.source_url)
FROM report_subjects AS subject
JOIN disclosure_reports AS report ON report.id = subject.report_id
WHERE expense.report_subject_id = subject.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE officer_hospitality_expenses AS expense
SET source_url = COALESCE(NULLIF(subject.hospitality_source_url, ''), NULLIF(subject.source_url, ''), report.source_url)
FROM report_subjects AS subject
JOIN disclosure_reports AS report ON report.id = subject.report_id
WHERE expense.report_subject_id = subject.id
  AND NULLIF(expense.source_url, '') IS NULL;

UPDATE officer_travel_expenses AS expense
SET source_url = COALESCE(NULLIF(subject.travel_source_url, ''), NULLIF(subject.source_url, ''), report.source_url)
FROM report_subjects AS subject
JOIN disclosure_reports AS report ON report.id = subject.report_id
WHERE expense.report_subject_id = subject.id
  AND NULLIF(expense.source_url, '') IS NULL;

ALTER TABLE mp_contract_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE mp_hospitality_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE mp_travel_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE administration_contract_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE administration_hospitality_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE administration_travel_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE officer_contract_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE officer_hospitality_expenses ALTER COLUMN source_url SET NOT NULL;
ALTER TABLE officer_travel_expenses ALTER COLUMN source_url SET NOT NULL;

COMMIT;