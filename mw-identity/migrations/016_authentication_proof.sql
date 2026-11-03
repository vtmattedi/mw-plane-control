ALTER TABLE identity_sessions ADD COLUMN auth_proof TEXT NOT NULL DEFAULT '';
ALTER TABLE identity_authorization_codes ADD COLUMN auth_proof TEXT NOT NULL DEFAULT '';
ALTER TABLE identity_access_tokens ADD COLUMN auth_proof TEXT NOT NULL DEFAULT '';
-- Never relabel legacy TOTP-only sessions as MFA.
DELETE FROM identity_sessions WHERE auth_method='totp';
CREATE TABLE identity_mfa_challenges (
 token_hash TEXT PRIMARY KEY,
 binding_hash TEXT NOT NULL,
 user_id TEXT NOT NULL REFERENCES identity_users(id) ON DELETE CASCADE,
 password_fingerprint TEXT NOT NULL,
 return_to TEXT NOT NULL,
 expires_at_unix INTEGER NOT NULL,
 attempts INTEGER NOT NULL DEFAULT 0,
 consumed INTEGER NOT NULL DEFAULT 0 CHECK(consumed IN (0,1))
);
