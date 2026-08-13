CREATE TABLE identity_applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    client_id TEXT NOT NULL UNIQUE,
    client_secret_hash TEXT NULL,
    application_type TEXT NOT NULL
        CHECK (application_type IN ('confidential', 'public')),
    authentication_policy TEXT NOT NULL DEFAULT 'standard',
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    archived_at_unix INTEGER NULL,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER NOT NULL
);

CREATE TABLE identity_application_redirect_uris (
    application_id TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL,
    PRIMARY KEY (application_id, redirect_uri),
    FOREIGN KEY (application_id)
        REFERENCES identity_applications(id)
        ON DELETE CASCADE
);

CREATE TABLE identity_application_post_logout_redirect_uris (
    application_id TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL,
    PRIMARY KEY (application_id, redirect_uri),
    FOREIGN KEY (application_id)
        REFERENCES identity_applications(id)
        ON DELETE CASCADE
);

CREATE TABLE identity_application_scopes (
    application_id TEXT NOT NULL,
    scope TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL,
    PRIMARY KEY (application_id, scope),
    FOREIGN KEY (application_id)
        REFERENCES identity_applications(id)
        ON DELETE CASCADE
);

CREATE TABLE identity_application_claims (
    application_id TEXT NOT NULL,
    claim TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL,
    PRIMARY KEY (application_id, claim),
    FOREIGN KEY (application_id)
        REFERENCES identity_applications(id)
        ON DELETE CASCADE
);

CREATE TABLE identity_application_audiences (
    application_id TEXT NOT NULL,
    audience TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL,
    PRIMARY KEY (application_id, audience),
    FOREIGN KEY (application_id)
        REFERENCES identity_applications(id)
        ON DELETE CASCADE
);
