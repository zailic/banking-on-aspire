CREATE TABLE IF NOT EXISTS processed_events (
    event_id TEXT PRIMARY KEY CHECK (event_id <> ''),
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS transactions (
    transaction_id TEXT PRIMARY KEY CHECK (transaction_id <> ''),
    user_id TEXT NOT NULL CHECK (user_id <> ''),
    account_name TEXT NOT NULL CHECK (account_name <> ''),
    counterparty TEXT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    amount_units BIGINT NOT NULL,
    amount_nanos INTEGER NOT NULL CHECK (amount_nanos BETWEEN 0 AND 999999999),
    direction TEXT NOT NULL CHECK (direction IN ('debit', 'credit')),
    reference TEXT NOT NULL,
    source_event_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS transactions_user_created_idx
    ON transactions (user_id, created_at DESC, transaction_id DESC);
