-- Existing clients require an operator-reviewed access cutover. New clients opt in at creation.
ALTER TABLE identity_applications ADD COLUMN access_policy_v1 INTEGER NOT NULL DEFAULT 0 CHECK(access_policy_v1 IN (0,1));
