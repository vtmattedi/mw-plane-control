CREATE TABLE bff_oidc_transactions (
    id TEXT PRIMARY KEY,
    state_hash TEXT NOT NULL UNIQUE,
    nonce_hash TEXT NOT NULL,
    pkce_verifier_ciphertext BLOB NOT NULL,
    pkce_verifier_nonce BLOB NOT NULL,
    encryption_key_version INTEGER NOT NULL,
    return_to TEXT NOT NULL,
    expires_at_unix INTEGER NOT NULL,
    consumed_at_unix INTEGER NULL,
    created_at_unix INTEGER NOT NULL
);

CREATE INDEX idx_bff_oidc_transactions_expires
    ON bff_oidc_transactions(expires_at_unix);

CREATE TABLE bff_admin_sessions (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_hash TEXT NOT NULL,
    identity_issuer TEXT NOT NULL,
    identity_subject TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL,
    auth_time_unix INTEGER NOT NULL,
    expires_at_unix INTEGER NOT NULL,
    created_at_unix INTEGER NOT NULL,
    last_seen_at_unix INTEGER NOT NULL
);

CREATE INDEX idx_bff_admin_sessions_subject
    ON bff_admin_sessions(identity_subject);

CREATE INDEX idx_bff_admin_sessions_expires
    ON bff_admin_sessions(expires_at_unix);
