CREATE TABLE IF NOT EXISTS accounts (
    account_id TEXT PRIMARY KEY CHECK (account_id <> ''),
    user_id TEXT NOT NULL REFERENCES users(user_id),
    display_name TEXT NOT NULL CHECK (display_name <> ''),
    account_type TEXT NOT NULL CHECK (account_type IN ('checking', 'savings')),
    status TEXT NOT NULL CHECK (status IN ('open', 'frozen', 'closed')),
    currency_code CHAR(3) NOT NULL,
    balance_units BIGINT NOT NULL,
    balance_nanos INTEGER NOT NULL CHECK (balance_nanos BETWEEN -999999999 AND 999999999),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    etag TEXT NOT NULL CHECK (etag <> '')
);

CREATE INDEX IF NOT EXISTS accounts_user_status_idx
    ON accounts (user_id, status, account_id);
