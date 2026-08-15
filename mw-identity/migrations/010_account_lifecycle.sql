ALTER TABLE identity_applications
    ADD COLUMN delete_callback_uri TEXT NULL;

ALTER TABLE identity_applications
    ADD COLUMN allow_public_signup INTEGER NOT NULL DEFAULT 0
        CHECK (allow_public_signup IN (0, 1));

ALTER TABLE identity_users
    ADD COLUMN username TEXT NOT NULL DEFAULT '';

ALTER TABLE identity_users
    ADD COLUMN username_normalized TEXT NULL;

ALTER TABLE identity_users
    ADD COLUMN account_type TEXT NOT NULL DEFAULT 'workforce'
        CHECK (account_type IN ('workforce', 'public'));

ALTER TABLE identity_users
    ADD COLUMN email_verified_at_unix INTEGER NULL;

ALTER TABLE identity_users
    ADD COLUMN deletion_requested_at_unix INTEGER NULL;

CREATE UNIQUE INDEX idx_identity_users_username_normalized
    ON identity_users(username_normalized)
    WHERE username_normalized IS NOT NULL;
