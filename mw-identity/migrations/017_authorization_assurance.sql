ALTER TABLE identity_authorization_codes ADD COLUMN requested_acr TEXT NOT NULL DEFAULT '';
ALTER TABLE identity_authorization_codes ADD COLUMN max_age_seconds INTEGER NOT NULL DEFAULT -1;
-- Unfinished legacy codes contain no versioned proof. Require reauthentication.
DELETE FROM identity_authorization_codes;
