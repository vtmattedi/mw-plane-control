-- Step 12: strong authentication, TOTP fallback, recovery, and assurance time.
ALTER TABLE identity_sessions
    ADD COLUMN auth_time_unix INTEGER NOT NULL DEFAULT 0;

UPDATE identity_sessions
SET auth_time_unix = created_at_unix
WHERE auth_time_unix = 0;

ALTER TABLE identity_users
    ADD COLUMN recovery_pending INTEGER NOT NULL DEFAULT 0
        CHECK (recovery_pending IN (0, 1));

CREATE TABLE identity_totp_credentials (
    user_id TEXT PRIMARY KEY,
    secret_ciphertext TEXT NOT NULL,
    secret_nonce TEXT NOT NULL,
    secret_key_version INTEGER NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    last_accepted_step INTEGER NOT NULL DEFAULT -1,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL,
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE TABLE identity_recovery_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN ('pending', 'consumed', 'cancelled')),
    expires_at_unix INTEGER NOT NULL,
    consumed_at_unix INTEGER NULL,
    created_at_unix INTEGER NOT NULL,
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_identity_recovery_user_status
    ON identity_recovery_requests(user_id, status);

CREATE INDEX idx_identity_recovery_expires
    ON identity_recovery_requests(expires_at_unix);
