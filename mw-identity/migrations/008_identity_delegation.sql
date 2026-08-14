ALTER TABLE identity_sessions
    ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE identity_authorization_codes
    ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE identity_access_tokens
    ADD COLUMN auth_time_unix INTEGER NOT NULL DEFAULT 0;

ALTER TABLE identity_access_tokens
    ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'unknown';

ALTER TABLE identity_access_tokens
    ADD COLUMN parent_token_id TEXT NULL;

CREATE INDEX idx_identity_access_tokens_audience_expires
    ON identity_access_tokens(audience, expires_at_unix);
