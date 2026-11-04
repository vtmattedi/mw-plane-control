-- Breaking protocol cutover, without deleting account, consent, assignment or
-- recovery/outbox history. Deprecated registry columns remain inert evidence.
INSERT INTO identity_audit_events
 (id,event_type,status,severity,reason_code,metadata_json,created_at_unix)
VALUES ('aud_federation_v1_' || lower(hex(randomblob(16))),
 'identity.federation.contract_migrated','success','warning','federation_v1',
 json_object('contract','1.0','access_codes_invalidated',(SELECT COUNT(*) FROM identity_authorization_codes),
             'access_tokens_invalidated',(SELECT COUNT(*) FROM identity_access_tokens),
             'recovery_requests_imported',(SELECT COUNT(*) FROM identity_users u
               WHERE u.deletion_requested_at_unix IS NOT NULL AND NOT EXISTS
                 (SELECT 1 FROM identity_deletion_operations o WHERE o.subject=u.id))),
 unixepoch());
-- A request-bound deletion after migration 018 may have frozen its identity
-- without an outbox. Preserve it for evidence-backed operator reconciliation;
-- there is no recoverable old event ID to replay automatically.
INSERT INTO identity_deletion_operations(id,subject,requested_at_unix,status)
 SELECT 'del_import_v1_' || lower(hex(randomblob(16))),u.id,u.deletion_requested_at_unix,'legacy_review_required'
 FROM identity_users u WHERE u.deletion_requested_at_unix IS NOT NULL
 AND NOT EXISTS (SELECT 1 FROM identity_deletion_operations o WHERE o.subject=u.id);
-- These are transient credentials, not user data. Consumers must start new
-- state/nonce/PKCE transactions against the single v1 assurance/access model.
DELETE FROM identity_authorization_codes;
DELETE FROM identity_access_tokens;
