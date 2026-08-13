CREATE TABLE identity_webauthn_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    rpid TEXT NOT NULL,
    user_handle BLOB NOT NULL,
    created_at TEXT NOT NULL DEFAULT (
        strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    ),
    UNIQUE (user_id, rpid),
    UNIQUE (rpid, user_handle),
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE TABLE identity_webauthn_credentials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    rpid TEXT NOT NULL,
    credential_id BLOB NOT NULL,
    credential_ciphertext TEXT NOT NULL,
    credential_nonce TEXT NOT NULL,
    credential_key_version INTEGER NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    transports_json TEXT NOT NULL DEFAULT '[]',
    aaguid BLOB,
    sign_count INTEGER NOT NULL DEFAULT 0,
    clone_warning INTEGER NOT NULL DEFAULT 0 CHECK (clone_warning IN (0, 1)),
    backup_eligible INTEGER NOT NULL DEFAULT 0 CHECK (backup_eligible IN (0, 1)),
    backup_state INTEGER NOT NULL DEFAULT 0 CHECK (backup_state IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (
        strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    ),
    last_used_at TEXT,
    revoked_at TEXT,
    UNIQUE (rpid, credential_id),
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_identity_webauthn_credentials_user
    ON identity_webauthn_credentials(user_id, rpid);

CREATE TABLE identity_webauthn_ceremonies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT NOT NULL UNIQUE,
    user_id TEXT,
    rpid TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN ('registration', 'authentication')),
    session_ciphertext TEXT NOT NULL,
    session_nonce TEXT NOT NULL,
    session_key_version INTEGER NOT NULL,
    label TEXT NOT NULL DEFAULT '',
    expires_at_unix INTEGER NOT NULL,
    consumed_at_unix INTEGER,
    created_at TEXT NOT NULL DEFAULT (
        strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
    ),
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_identity_webauthn_ceremonies_expiry
    ON identity_webauthn_ceremonies(expires_at_unix);
