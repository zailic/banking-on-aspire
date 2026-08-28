ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS account_number TEXT;

-- Preserve development rows created before account numbers were introduced.
-- New accounts must receive their number during provisioning.
UPDATE accounts
SET account_number = 'LEGACY-' || account_id
WHERE account_number IS NULL OR account_number = '';

ALTER TABLE accounts
    ALTER COLUMN account_number SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS accounts_account_number_uidx
    ON accounts (account_number);
