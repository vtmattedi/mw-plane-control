-- Known applications survive expiry/disconnection; history is not consent.
CREATE TABLE identity_application_consents (
  user_id TEXT NOT NULL REFERENCES identity_users(id) ON DELETE CASCADE,
  application_id TEXT NOT NULL REFERENCES identity_applications(id) ON DELETE CASCADE,
  active INTEGER NOT NULL DEFAULT 0 CHECK(active IN (0,1)),
  version INTEGER NOT NULL DEFAULT 1 CHECK(version > 0),
  scopes_json TEXT NOT NULL DEFAULT '[]',
  created_at_unix INTEGER NOT NULL,
  updated_at_unix INTEGER NOT NULL,
  PRIMARY KEY(user_id, application_id)
);
INSERT OR IGNORE INTO identity_application_consents
  (user_id,application_id,created_at_unix,updated_at_unix)
SELECT user_id,application_id,created_at_unix,updated_at_unix FROM identity_application_assignments;
INSERT OR IGNORE INTO identity_application_consents
  (user_id,application_id,created_at_unix,updated_at_unix)
SELECT t.user_id,p.id,MIN(t.created_at_unix),MAX(t.created_at_unix)
FROM identity_access_tokens t JOIN identity_applications p ON p.client_id=t.client_id
GROUP BY t.user_id,p.id;
INSERT OR IGNORE INTO identity_application_consents
  (user_id,application_id,created_at_unix,updated_at_unix)
SELECT t.user_id,p.id,MIN(t.created_at_unix),MAX(t.created_at_unix)
FROM identity_authorization_codes t JOIN identity_applications p ON p.client_id=t.client_id
GROUP BY t.user_id,p.id;
ALTER TABLE identity_authorization_codes ADD COLUMN consent_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE identity_access_tokens ADD COLUMN consent_version INTEGER NOT NULL DEFAULT 0;
-- Legacy external-client credentials require explicit re-consent.
DELETE FROM identity_authorization_codes WHERE client_id IN
 (SELECT client_id FROM identity_applications WHERE internal=0);
DELETE FROM identity_access_tokens WHERE client_id IN
 (SELECT client_id FROM identity_applications WHERE internal=0);

-- Explicitly trusted internal clients retain their preexisting access.
UPDATE identity_application_consents SET active=1 WHERE application_id IN (SELECT id FROM identity_applications WHERE internal=1);

-- Preserve existing explicitly internal credentials, binding them to migrated
-- trusted consent rather than leaving an untracked version-zero bypass.
UPDATE identity_authorization_codes SET consent_version=(SELECT c.version
 FROM identity_application_consents c JOIN identity_applications p ON p.id=c.application_id
 WHERE c.user_id=identity_authorization_codes.user_id AND p.client_id=identity_authorization_codes.client_id)
 WHERE client_id IN (SELECT client_id FROM identity_applications WHERE internal=1);
UPDATE identity_access_tokens SET consent_version=(SELECT c.version
 FROM identity_application_consents c JOIN identity_applications p ON p.id=c.application_id
 WHERE c.user_id=identity_access_tokens.user_id AND p.client_id=identity_access_tokens.client_id)
 WHERE client_id IN (SELECT client_id FROM identity_applications WHERE internal=1);
