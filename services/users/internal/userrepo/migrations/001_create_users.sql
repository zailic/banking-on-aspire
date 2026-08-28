CREATE TABLE IF NOT EXISTS users (
    user_id TEXT PRIMARY KEY CHECK (user_id <> ''),
    keycloak_subject TEXT NOT NULL UNIQUE CHECK (keycloak_subject <> ''),
    username TEXT NOT NULL UNIQUE CHECK (username <> ''),
    display_name TEXT NOT NULL CHECK (display_name <> ''),
    email TEXT,
    status TEXT NOT NULL CHECK (status IN ('active', 'suspended', 'closed')),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    etag TEXT NOT NULL CHECK (etag <> '')
);

CREATE INDEX IF NOT EXISTS users_status_username_idx
    ON users (status, username, user_id);

