CREATE TABLE IF NOT EXISTS contacts (
    user_id TEXT NOT NULL CHECK (user_id <> ''),
    contact_id TEXT NOT NULL CHECK (contact_id <> ''),
    display_name TEXT NOT NULL CHECK (display_name <> ''),
    destination_type TEXT NOT NULL CHECK (destination_type IN ('internal', 'external')),
    internal_account TEXT,
    external_routing_number TEXT,
    external_account_number TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    etag TEXT NOT NULL CHECK (etag <> ''),
    PRIMARY KEY (user_id, contact_id),
    CONSTRAINT contacts_destination_check CHECK (
        (destination_type = 'internal'
            AND internal_account IS NOT NULL
            AND external_routing_number IS NULL
            AND external_account_number IS NULL)
        OR
        (destination_type = 'external'
            AND internal_account IS NULL
            AND external_routing_number IS NOT NULL
            AND external_account_number IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS contacts_user_display_name_idx
    ON contacts (user_id, display_name, contact_id);
