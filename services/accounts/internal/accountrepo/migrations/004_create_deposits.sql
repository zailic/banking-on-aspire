CREATE TABLE IF NOT EXISTS deposits (
    deposit_id TEXT PRIMARY KEY CHECK (deposit_id <> ''),
    account_id TEXT NOT NULL REFERENCES accounts(account_id),
    currency_code CHAR(3) NOT NULL,
    amount_units BIGINT NOT NULL,
    amount_nanos INTEGER NOT NULL CHECK (amount_nanos BETWEEN 0 AND 999999999),
    reference TEXT NOT NULL,
    request_id TEXT NOT NULL CHECK (request_id <> ''),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (account_id, request_id)
);

CREATE INDEX IF NOT EXISTS deposits_account_created_idx
    ON deposits (account_id, created_at DESC, deposit_id);
