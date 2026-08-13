CREATE TABLE identity_users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    email_normalized TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    is_superadmin INTEGER NOT NULL DEFAULT 0 CHECK (is_superadmin IN (0, 1)),
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL
);

CREATE TABLE identity_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_hash TEXT NOT NULL,
    expires_at_unix INTEGER NOT NULL,
    created_at_unix INTEGER NOT NULL,
    last_seen_at_unix INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES identity_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_identity_sessions_user_id
    ON identity_sessions(user_id);

CREATE INDEX idx_identity_sessions_expires_at_unix
    ON identity_sessions(expires_at_unix);

CREATE TABLE identity_audit_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    status TEXT NOT NULL,
    severity TEXT NOT NULL,
    user_id TEXT NULL,
    remote_ip TEXT NULL,
    reason_code TEXT NULL,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at_unix INTEGER NOT NULL,
    FOREIGN KEY (user_id) REFERENCES identity_users(id) ON DELETE SET NULL
);

CREATE INDEX idx_identity_audit_events_created_at_unix
    ON identity_audit_events(created_at_unix);

CREATE INDEX idx_identity_audit_events_user_id
    ON identity_audit_events(user_id);
