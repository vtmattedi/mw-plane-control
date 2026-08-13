ALTER TABLE bff_admin_sessions
    ADD COLUMN identity_access_token_ciphertext BLOB;

ALTER TABLE bff_admin_sessions
    ADD COLUMN identity_access_token_nonce BLOB;

ALTER TABLE bff_admin_sessions
    ADD COLUMN identity_access_token_key_version INTEGER NOT NULL DEFAULT 0;
