# Backup and restore implementation plan

Status: implementation started. The first increment provides Identity-only encrypted archive creation, validation, and isolated preparation; see `runbook-product-backup.md`. The host runner, BFF recovery, live replacement/rollback, API/UI, and scheduling remain to be implemented.

## Goal and agreed recovery contract

Recover existing users, global superadmins, application roles, and authentication credentials after persistence loss. Preserve original user IDs and application IDs so downstream identity references remain valid.

Version one requires the operator to retain and reuse the original deployment `.env`. Never include `.env` or encryption keyrings in the database archive. Store them securely and separately. If keys rotate, retain all versions referenced by retained backups. Validate effective configuration and actual decryption rather than requiring byte-identical `.env` files.

Keep issuer, WebAuthn RP ID/origins, BFF client registration/secret, and required encryption keys compatible with the backup. Domain migration and selective user merging are separate future workflows.

## Scope

First release backs up Identity and BFF as a control-plane recovery set. Identity contains durable accounts and authorization; the current BFF tables contain authentication transactions, sessions, and delegated token caches. Snapshot BFF for safety and diagnostics, but invalidate its authentication state before serving a restored deployment.

The Workspace page must explicitly say which services are covered. OAuth and Mail databases are not covered until their own persistence adapters and coordinated recovery procedures exist.

Preserve:

- All user fields, including password hashes, enabled state, account type, verification/deletion state, recovery restrictions, and `is_superadmin`.
- Application registrations, policies, redirects, scopes, claims, audiences, and assignments with their roles.
- WebAuthn user handles, credential identifiers, encrypted credential state, revocation state, and TOTP credentials.
- Signing-key records and Identity audit history.

Invalidate on every restore:

- Identity sessions, authorization codes, access/delegated tokens, and WebAuthn ceremonies.
- BFF OIDC transactions, admin sessions, and delegated token caches.
- Pending recovery capabilities. Preserve `recovery_pending`; an operator must start a new recovery for affected users.

Review downstream JWT validation and token caches before release: clearing local token rows alone does not guarantee rejection by a downstream service validating a JWT offline. Define a supported revocation/expiry procedure and keep maintenance in place for its required interval.

## Architecture

Use the existing Go SQLite snapshot and integrity primitives as a foundation, not the construction-step PowerShell scripts as the product API.

Implement an operator command and a host operations runner that remain available while gateway, Identity, and BFF are stopped. The runner owns a separate job journal and archive directory outside the databases being restored. BFF submits bounded jobs to this runner over a private authenticated channel; it never executes arbitrary shell commands supplied by requests. Do not mount the Docker socket into BFF or Identity.

Start with a host runner supporting the existing Compose deployment. Keep its interface independent of Compose so another deployment adapter can be added later. Use argument arrays, fixed commands, allowlisted services and workspace paths. Accept backup IDs rather than client-supplied filesystem paths.

Journal jobs durably with states such as queued, validating, snapshotting, prepared, replacing, checking, succeeded, failed, and rollback-required. Only one backup/restore job may operate on the deployment at a time. Recover safely after a process restart, including a crash between replacement of the two databases.

## Archive format

Define `mw-control-plane-recovery-v1` with Identity and BFF snapshots and a manifest. Include format/schema versions, build identity, UTC timestamp, deployment identifier, component coverage, file lengths/checksums, required encryption-key versions, critical nonsecret configuration, and user/root/application/assignment counts.

Encrypt and authenticate the complete archive using a reviewed library and an independently retained recovery passphrase/key. Do not derive archive encryption from Identity/BFF runtime keyrings. Keep detailed metadata inside the encrypted archive. Pass recovery secrets through a secure prompt or bounded request body, never command-line arguments or logs; never persist the passphrase in the job journal.

Configuration checks should compare nonsecret settings and test decryption. Do not publish hashes of `.env` or low-entropy secrets. A checksum detects corruption; authenticated encryption protects the archive from tampering. Reject unsupported formats, unsafe archive paths, duplicate entries, and excessive compressed/uncompressed sizes before extraction.

## Backup workflow

1. Authorize the request and acquire the deployment job lock.
2. Create SQLite-aware online snapshots using `VACUUM INTO`; do not copy live database/WAL files. Document that separately taken snapshots are not a distributed transaction. They are sufficient for this release because BFF authentication state is discarded during recovery.
3. Verify database integrity and foreign keys; collect schema versions and required key versions from stored records.
4. Build, encrypt, and authenticate the archive. Publish it only after completion; delete partial output and plaintext temporary files.
5. Record the result in the independent job journal and Identity audit. Offer a bounded authenticated download.

## Restore workflow

1. Upload/decrypt into an isolated restricted staging directory. Validate archive structure, integrity, schema/build compatibility, configuration, and decryption of all encrypted security records.
2. Produce a preview bound to the immutable archive digest: snapshot time, component coverage, counts, global root status, recovery-pending users, and rollback consequences. Normal restore requires an enabled global root with usable authentication material; operator recovery remains available for exceptional cases.
3. Require explicit confirmation of that validated archive and recent WebAuthn for browser-triggered execution. Revalidate immediately before execution to detect configuration changes.
4. Acquire the lock, enable maintenance, stop writes and services, and create a consistent safety backup of the current databases. Preserve the pre-restore journal/audit evidence independently.
5. Prepare replacement copies, apply the documented supported migrations, invalidate transient authentication state, and verify again. Never run migrations on the only copy of the input backup.
6. Replace both databases through checked paths/volumes while services are stopped, removing obsolete WAL/SHM companions. Record each completed replacement in the journal. Do not delete named volumes.
7. Restart and check readiness, usable signing keys, credential decryption, root presence, and application registration/assignments. On failure, keep maintenance active and offer journal-driven rollback to the safety copies; do not report success or silently resume a partial restore.
8. Finish automated checks, expose login for verification, and mark the operation as awaiting administrator verification. Require a fresh global-superadmin WebAuthn login and an authorized administration request to complete verification. Record success in both the independent journal and restored Identity audit.

Passkey signature counters and TOTP replay counters can move backward with a database rollback. Include explicit drills and define handling without disabling clone/replay checks globally. A restore can also resurrect users, assignments, and credentials deleted or revoked after the backup: show this in the preview and require review before normal privileged use.

## Authorization and UI/API

Add Workspace -> Backup & Restore at `/workspace/backups`. Provide coverage, manual creation/download, upload/validation preview, explicit restore confirmation, job history, and administrator-verification status. Once execution starts, explain the maintenance interruption and require login again afterward. The operator CLI is the source of recovery progress if the web app is unavailable.

Add BFF routes under `/api/workspace/backups` and `/api/workspace/restores` for listing, creation, download, validation, confirmation, and job status. Validate session/CSRF as appropriate, apply request and archive limits, and require recent WebAuthn for creation, download, validation, and execution.

Authorize against the current global `identity_users.is_superadmin` state in Identity. Existing BFF sessions do not carry that flag, and application `superadmin` permissions must not be treated as global root. Add a narrowly scoped Identity capability check tied to the authenticated subject and recheck it before job execution. UI visibility is not authorization.

## Delivery phases and acceptance criteria

### 1. Recovery core and operator CLI

Implement archive creation/validation, environment compatibility checks, transient-state invalidation, host runner, locking, journal, maintenance, safety copies, restore, and rollback. Update disaster recovery and administrator recovery runbooks.

Acceptance: restore into a disposable deployment using the saved `.env`; original user IDs/password hashes/global roots/assignments survive; credentials decrypt; old sessions and recovery links fail; a root can log in and perform privileged work. Wrong keys, corrupt archives, incompatible schemas, and missing roots fail before replacement. Inject failures between database replacements and after restart to demonstrate recovery from the journal.

### 2. Product API and Workspace UI

Implement the current-root capability check, BFF runner client/routes, recent-WebAuthn gates, bounded upload/download, preview and confirmation, job history, and frontend translations. Add the runner deployment/configuration documentation.

Acceptance: users, application admins, and application superadmins cannot invoke recovery routes; stale/password/TOTP sessions require step-up; CSRF and forged backup IDs are rejected; jobs survive BFF downtime and cannot execute concurrently; successful recovery requires fresh login.

### 3. Scheduled off-host backups

Add operator-managed schedule, storage adapter, retention, alerting, and restore-drill reporting. Choose the off-host storage target during this phase. Store scheduled archive encryption credentials separately from database backups and redact them from logs.

Acceptance: failed uploads never count as successful off-host backups; retention removes only completed eligible archives and protects the current safety backup; alerts report failures; a downloaded off-host archive passes a full restore drill.

### 4. OAuth and Mail coverage

Inventory each service's durable data, secrets, and user-ID dependencies. Implement service-specific snapshot/validation adapters, a write-quiescence strategy where necessary, and coordinated rollback. Only then label the feature as covering the entire deployment.

## Source documents and code

- `docs/MW-Identity-Implementation-guide.md`, sections 13, 34, 37, and 38.
- `docs/construction_steps/step-06-identity-administration.md` for global root versus application roles.
- `docs/dev_docs/runbook-disaster-recovery.md` and `runbook-identity-recovery.md`.
- `mw-identity/migrations/002_identity_local_auth.sql`, `006_identity_admin.sql`, `007_identity_webauthn.sql`, `009_identity_strong_auth.sql`, and `010_account_lifecycle.sql`.
- `mw-identity/internal/store/sqlite/operations.go` and `mw-bff/internal/store/sqlite/operations.go`.
- `mw-bff/internal/httpapi/strong_auth.go`, `router.go`, and `mw-web-admin/src/pages/AppShell.tsx`.
