ALTER TABLE identity_applications ADD COLUMN account_management_uri TEXT NOT NULL DEFAULT '';
ALTER TABLE identity_applications ADD COLUMN account_deletion_uri TEXT NOT NULL DEFAULT '';
