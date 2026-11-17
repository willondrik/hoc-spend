-- Stores the province or territory in the historical MP affiliation context.
--
-- mps.province is optional current-profile enrichment and cannot safely be
-- used for historic disclosures, former members, vacancies, or redistributed
-- districts. The report loader resolves and writes this column from its
-- version-controlled constituency reference before it creates a report subject.

BEGIN;

ALTER TABLE mp_affiliations
    ADD COLUMN IF NOT EXISTS province TEXT;

CREATE INDEX IF NOT EXISTS mp_affiliations_province_idx
    ON mp_affiliations (province);

COMMIT;