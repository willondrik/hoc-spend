-- Adds a strict advertising category for reportable advertising-account
-- analysis. This is a source-description proxy, not an official House account
-- balance: only descriptions explicitly identifying advertising or publicity
-- are included. Generic promotional items and broader communications remain in
-- their existing categories.

BEGIN;

ALTER TABLE mp_contract_expenses
    DROP CONSTRAINT IF EXISTS mp_contract_expenses_category_check,
    ADD CONSTRAINT mp_contract_expenses_category_check CHECK (category IN (
        'information_technology', 'advertising', 'communications',
        'professional_services', 'office_and_equipment', 'facilities',
        'printing_and_translation', 'training_and_events', 'travel', 'other'
    ));
ALTER TABLE administration_contract_expenses
    DROP CONSTRAINT IF EXISTS administration_contract_expenses_category_check,
    ADD CONSTRAINT administration_contract_expenses_category_check CHECK (category IN (
        'information_technology', 'advertising', 'communications',
        'professional_services', 'office_and_equipment', 'facilities',
        'printing_and_translation', 'training_and_events', 'travel', 'other'
    ));
ALTER TABLE officer_contract_expenses
    DROP CONSTRAINT IF EXISTS officer_contract_expenses_category_check,
    ADD CONSTRAINT officer_contract_expenses_category_check CHECK (category IN (
        'information_technology', 'advertising', 'communications',
        'professional_services', 'office_and_equipment', 'facilities',
        'printing_and_translation', 'training_and_events', 'travel', 'other'
    ));

UPDATE mp_contract_expenses
SET category = 'advertising'
WHERE lower(description) LIKE ANY (ARRAY['%advertising%', '%publicity%']);
UPDATE administration_contract_expenses
SET category = 'advertising'
WHERE lower(description) LIKE ANY (ARRAY['%advertising%', '%publicity%']);
UPDATE officer_contract_expenses
SET category = 'advertising'
WHERE lower(description) LIKE ANY (ARRAY['%advertising%', '%publicity%']);

COMMIT;