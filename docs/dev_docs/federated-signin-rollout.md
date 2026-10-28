# Federated sign-in rollout (2026-10-27)

Disconnect now revokes consent without deleting application data. Applications
own account deletion; whole-MW deletion remains a separate notification flow.

## Order and migration

1. Inventory registered clients and validate `internal` classifications. Deploy
   WOI's authenticated legacy-disconnect ignore safeguard first. Coordinate the
   portal release so old copy does not promise deletion after this change. The new receiver requires a signed reason=account_deleted for destructive callbacks. An old provider does not sign reason: its whole-account callback receives 401 and remains pending until the provider is upgraded and an administrator retries. Use a coordinated maintenance window; do not weaken signature checks to accept legacy destructive callbacks. A legacy disconnect is safely ignored even when its reason claim is absent.
2. Stop Identity writers and take a SQLite backup through the established backup
   command/runbook. Rehearse restore and migration on an isolated copy. Do not
   run old and new binaries concurrently against the same store.
3. Start the updated Identity binary. Migration
   `mw-identity/migrations/013_application_consent.sql` runs transactionally via
   the existing migration runner. It adds consent/history and credential-version
   columns, copies known assignment/token associations as inactive history and
   invalidates external-client codes/tokens lacking consent binding. All old
   administrator assignments and denials remain. External clients require one
   explicit consent at their next authorization; app-local sessions survive.
4. Explicitly internal clients retain trusted initial access: existing history
   is marked active, and new internal-client connections bootstrap consent after
   normal eligibility checks. An explicit disconnect overrides this bootstrap;
   re-entry then requires consent. Verify admin/BFF/mail/OAuth flows before
   allowing public traffic. Never mark arbitrary external clients internal.
5. Deploy WOI's additive `account_deletion_intents` entity/table and frontend.
   Its schema is in `working-on-it/backend/sql/001_account_deletion_intents.sql`.
   Existing TypeORM synchronization remains enabled; stop app traffic, back up
   PostgreSQL, and validate synchronization on an isolated restored database.
6. Publish corresponding portal/admin/showcase documentation and illustrations
   only after local checks and separately authorized live acceptance.

Connection display comes from active consent, not expiring tokens. Historical
consent and administrator assignments remain whole-account deletion targets.
Permanent client purge is blocked while consent history exists. Previously lost
token-only associations cannot be reconstructed by this migration.

## Recovery and rollback

Revocation advances the consent version atomically with credential revocation.
Stale code/token/delegation writes and reads fail against the current version.
Interrupted consent forms expire in ten minutes and are lost on process restart;
users start a new sign-in. Durable consent survives restart.

Keep additive tables/columns and the consumer ignore-disconnect safeguard during
rollback. An old provider reinstates destructive disconnect behavior and omits
consent enforcement; disable affected portal actions rather than treating an old
binary as a compatible rollback. Restore backups only with the established
deletion-replay precautions and traffic stopped. Do not silently restore revoked
credentials or completed deletions.

Whole-account delivery still has its existing synchronous retry/operation-ID
limitations and no asynchronous completion tracking. This release does not
guarantee immediate remote logout, third-party erasure, or backup deletion.

## Release checkpoint

Use actual Identity plus an isolated WOI database: consent/onboard; disconnect
with WOI open; reconnect to the same profile; delete inside WOI after fresh
authentication and final confirmation; register a new baseline profile; replay
legacy disconnect and confirm it survives. Check cancellation, explicit deny,
scope expansion, callback unavailability, stale requests and file-cleanup retry.
Verify whole-account deletion separately with a disposable identity. Preserve
other-client access. Local evidence does not certify production registration.
