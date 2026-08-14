CREATE TABLE bff_delegated_tokens (
    id TEXT PRIMARY KEY,
    admin_session_id TEXT NOT NULL,
    audience TEXT NOT NULL,
    scope TEXT NOT NULL,
    token_ciphertext BLOB NOT NULL,
    token_nonce BLOB NOT NULL,
    encryption_key_version INTEGER NOT NULL,
    expires_at_unix INTEGER NOT NULL,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL,
    UNIQUE (admin_session_id, audience, scope),
    FOREIGN KEY (admin_session_id)
        REFERENCES bff_admin_sessions(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_bff_delegated_tokens_expires
    ON bff_delegated_tokens(expires_at_unix);
