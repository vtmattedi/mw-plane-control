-- Internal applications run behind the MW BFF and keep no user data of their own.
-- Every other application must configure delete_callback_uri; existing ones without
-- it keep working and are flagged until their next edit.
ALTER TABLE identity_applications
    ADD COLUMN internal INTEGER NOT NULL DEFAULT 0
        CHECK (internal IN (0, 1));
