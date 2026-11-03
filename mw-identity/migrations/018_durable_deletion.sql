CREATE TABLE identity_deletion_operations (
 id TEXT PRIMARY KEY,
 subject TEXT NOT NULL UNIQUE,
 requested_at_unix INTEGER NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','legacy_review_required','completed')),
 capability_hash TEXT UNIQUE,
 capability_expires_at_unix INTEGER NOT NULL DEFAULT 0,
 completed_at_unix INTEGER
);
CREATE TABLE identity_deletion_deliveries (
 event_id TEXT PRIMARY KEY,
 operation_id TEXT NOT NULL REFERENCES identity_deletion_operations(id),
 application_id TEXT NOT NULL,
 client_id TEXT NOT NULL,
 application_name TEXT NOT NULL,
 callback_uri TEXT NOT NULL,
 status TEXT NOT NULL CHECK(status IN ('pending','delivering','confirmed_cleanup','no_current_mapping','exempt','failed','unresolved')),
 attempts INTEGER NOT NULL DEFAULT 0,
 next_attempt_at_unix INTEGER NOT NULL,
 retry_started_at_unix INTEGER NOT NULL,
 lease_token TEXT NOT NULL DEFAULT '',
 lease_until_unix INTEGER NOT NULL DEFAULT 0,
 error_code TEXT NOT NULL DEFAULT '',
 UNIQUE(operation_id,application_id)
);
CREATE INDEX idx_deletion_deliveries_due ON identity_deletion_deliveries(status,next_attempt_at_unix,lease_until_unix);
-- Old callback IDs were not persisted. Preserve requests without inventing replayable events.
INSERT INTO identity_deletion_operations(id,subject,requested_at_unix,status)
 SELECT 'del_legacy_'||lower(hex(randomblob(16))),id,deletion_requested_at_unix,'legacy_review_required'
 FROM identity_users WHERE deletion_requested_at_unix IS NOT NULL;
-- Deletion is irreversible through ordinary enable/recovery paths.
CREATE TRIGGER identity_deleting_stays_disabled BEFORE UPDATE OF enabled ON identity_users
 WHEN NEW.deletion_requested_at_unix IS NOT NULL AND NEW.enabled=1
 BEGIN SELECT RAISE(ABORT,'identity deletion pending'); END;

UPDATE identity_users SET enabled=0 WHERE deletion_requested_at_unix IS NOT NULL;
