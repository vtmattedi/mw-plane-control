CREATE TABLE identity_signing_keys (
    kid TEXT PRIMARY KEY,
    alg TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'retired')),
    public_jwk_json TEXT NOT NULL,
    private_key_ciphertext BLOB NOT NULL,
    private_key_nonce BLOB NOT NULL,
    encryption_key_version INTEGER NOT NULL,
    created_at_unix INTEGER NOT NULL,
    retired_at_unix INTEGER NULL
);

CREATE UNIQUE INDEX idx_identity_signing_keys_one_active
    ON identity_signing_keys(status)
    WHERE status = 'active';

CREATE INDEX idx_identity_signing_keys_created_at
    ON identity_signing_keys(created_at_unix);
