ALTER TABLE identity_applications
    ADD COLUMN access_policy TEXT NOT NULL DEFAULT 'default_deny'
        CHECK (access_policy IN ('default_allow', 'default_deny'));

ALTER TABLE identity_application_assignments
    ADD COLUMN access_effect TEXT NOT NULL DEFAULT 'allow'
        CHECK (access_effect IN ('allow', 'deny'));
