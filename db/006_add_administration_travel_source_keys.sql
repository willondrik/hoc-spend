-- Preserves distinct Administration travel rows when the House reuses an
-- event_id within the same quarterly report. event_id remains the published
-- reference value; source_key is the scraper's durable row identity.
-- Run once after migrations 002-005.

BEGIN;

ALTER TABLE administration_travel_expenses
    ADD COLUMN IF NOT EXISTS source_key TEXT;

-- Rows imported before occurrence-aware source keys receive a stable legacy
-- identity. New scraper loads use a hash of the complete source row instead.
UPDATE administration_travel_expenses
SET source_key = 'legacy-administration-travel-' || id
WHERE source_key IS NULL OR source_key = '';

ALTER TABLE administration_travel_expenses
    ALTER COLUMN source_key SET NOT NULL;

ALTER TABLE administration_travel_expenses
    DROP CONSTRAINT IF EXISTS administration_travel_expenses_report_event_key;

ALTER TABLE administration_travel_expenses
    ADD CONSTRAINT administration_travel_expenses_report_source_key
    UNIQUE (report_id, source_key);

COMMIT;