-- Preserve event precision for max_age=0 ordering; JWT auth_time remains epoch seconds.
ALTER TABLE identity_sessions ADD COLUMN auth_time_nanos INTEGER NOT NULL DEFAULT 0;
