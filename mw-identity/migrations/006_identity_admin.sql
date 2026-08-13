CREATE TABLE identity_application_assignments (
    application_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL
        CHECK (role IN ('user', 'admin', 'superadmin')),
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL,
    PRIMARY KEY (application_id, user_id),
    FOREIGN KEY (application_id)
        REFERENCES identity_applications(id)
        ON DELETE CASCADE,
    FOREIGN KEY (user_id)
        REFERENCES identity_users(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_identity_application_assignments_user
    ON identity_application_assignments(user_id);

ALTER TABLE identity_access_tokens
    ADD COLUMN audience TEXT NOT NULL DEFAULT '';
