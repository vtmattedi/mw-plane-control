CREATE TABLE identity_authorization_codes (
    id TEXT PRIMARY KEY,
    code_hash TEXT NOT NULL UNIQUE,
    client_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    scope_json TEXT NOT NULL,
    code_challenge TEXT NOT NULL,
    nonce TEXT NULL,
    auth_time_unix INTEGER NOT NULL,
    expires_at_unix INTEGER NOT NULL,
    consumed_at_unix INTEGER NULL,
    created_at_unix INTEGER NOT NULL,
    FOREIGN KEY (client_id)
        REFERENCES identity_applications(client_id)
        ON DELETE CASCADE,
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_identity_authorization_codes_expires
    ON identity_authorization_codes(expires_at_unix);

CREATE TABLE identity_access_tokens (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    client_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    scope_json TEXT NOT NULL,
    expires_at_unix INTEGER NOT NULL,
    created_at_unix INTEGER NOT NULL,
    revoked_at_unix INTEGER NULL,
    FOREIGN KEY (client_id)
        REFERENCES identity_applications(client_id)
        ON DELETE CASCADE,
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_identity_access_tokens_expires
    ON identity_access_tokens(expires_at_unix);
