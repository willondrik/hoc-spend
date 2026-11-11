-- Separate the canonical public House member-page URL from the internal,
-- deterministic disclosure identity used to load historic MP spending rows.
-- Run once after migrations 002-009.

BEGIN;

ALTER TABLE mps
    ADD COLUMN IF NOT EXISTS source_identity TEXT;

-- Previous scraper runs stored synthetic source:// values in profile_url.
-- Move those values to the internal identity column and clear profile_url so it
-- can contain only the actual public House member profile URL.
UPDATE mps
SET source_identity = profile_url,
    profile_url = NULL
WHERE source_identity IS NULL
  AND profile_url LIKE 'source://ourcommons.ca/proactivedisclosure/mp/%';

-- Be defensive for rows created before synthetic identities existed. These rows
-- retain an opaque internal key without fabricating a public profile URL.
UPDATE mps
SET source_identity = 'source://ourcommons.ca/proactivedisclosure/mp/legacy/' || id::text
WHERE source_identity IS NULL;

ALTER TABLE mps
    ALTER COLUMN source_identity SET NOT NULL;

ALTER TABLE mps
    ADD CONSTRAINT mps_source_identity_key UNIQUE (source_identity);

COMMIT;