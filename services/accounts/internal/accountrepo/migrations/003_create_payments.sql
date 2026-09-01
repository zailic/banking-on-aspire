CREATE TABLE IF NOT EXISTS payments (
    payment_id TEXT PRIMARY KEY CHECK (payment_id <> ''),
    source_account_id TEXT NOT NULL REFERENCES accounts(account_id),
    beneficiary_user_id TEXT NOT NULL,
    beneficiary_contact_id TEXT NOT NULL,
    currency_code CHAR(3) NOT NULL,
    amount_units BIGINT NOT NULL,
    amount_nanos INTEGER NOT NULL CHECK (amount_nanos BETWEEN 0 AND 999999999),
    reference TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('completed')),
    request_id TEXT NOT NULL CHECK (request_id <> ''),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (source_account_id, request_id),
    FOREIGN KEY (beneficiary_user_id, beneficiary_contact_id)
        REFERENCES contacts(user_id, contact_id)
);

CREATE INDEX IF NOT EXISTS payments_source_created_idx
    ON payments (source_account_id, created_at DESC, payment_id);

CREATE TABLE IF NOT EXISTS account_outbox (
    event_id TEXT PRIMARY KEY CHECK (event_id <> ''),
    event_type TEXT NOT NULL CHECK (event_type <> ''),
    aggregate_name TEXT NOT NULL CHECK (aggregate_name <> ''),
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS account_outbox_unpublished_idx
    ON account_outbox (occurred_at, event_id)
    WHERE published_at IS NULL;
