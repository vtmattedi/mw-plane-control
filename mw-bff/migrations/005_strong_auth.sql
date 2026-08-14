ALTER TABLE bff_admin_sessions
    ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'unknown';
