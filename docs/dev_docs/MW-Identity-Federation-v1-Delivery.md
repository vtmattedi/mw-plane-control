# Federation v1 delivery record

The frozen contract remains normative. Each PR increment is a separate local commit. Deployment and consumer evidence remain separate from source implementation. Never advertise full compliance while any required live gate is unverified.

## PR-00

Recorded baseline, preserved frozen contract, copied supplied implementation plan and added the dependency inventory and A01–A28 baseline map. See the compliance document for exact test outcomes and unresolved live checks.

## PR-01

Migration 014 adds `access_policy_v1` without changing existing registrations. New applications created by the service use v1 rules: Public defaults allow, Private requires assignment, explicit deny wins, ordinary-app superadmins receive no exemption. SQL eligibility is shared across credential insert/read/consume paths. Existing legacy clients remain explicitly noncompliant until reviewed cutover, avoiding guesses about their intended signup-based access. Global registration controls app-initiated registration too; private-login signup invitations are hidden and restriction wording is bilingual. Direct global registration remains available.

Validation: Identity `go test ./...`, focused portal/store tests and `go vet ./...` pass. The new store integration test compares Go authorization and SQL issuance/read results for unassigned and explicitly denied superadmins and rejects signup bypass after cutover. No production database used.

Rollout: back up; perform the client audit in the compliance document; establish explicit Private assignments first; run `/app/federation-cutover --client-id <reviewed-client> --access public|private --reviewed`. The command atomically changes the reviewed policy and revokes existing client codes/tokens, with an audit record. Existing client ID/secret stays intact. Observe fresh authorization before the next cutover. Live audit is unverified here.

Rollback: revert code while retaining additive columns; restore the recorded access setting only through an operator-reviewed change. Old binaries do not honor `access_policy_v1`, so reverting may restore legacy signup/superadmin access bypasses. Do not use rollback as an implicit policy change. No schema downgrade or historical lifecycle replay.

## PR-02

Migration 015 adds optional account management/deletion navigation URLs. Registry create/read/update round trips preserve them; validation requires bounded absolute HTTPS, no credentials/fragments or identity/credential query fields, with documented HTTP localhost development navigation. Normal client forms distinguish OAuth type from Public/Private access and remove Strong, per-app signup and infrastructure controls in both languages. Normal API rejects infrastructure creation/promotion and privileged Identity registrations. Host-only `create-application --internal` remains the controlled bootstrap workflow and records provisioning audit. Existing infrastructure cannot be reclassified by an ordinary edit. Existing Strong fields are preserved and cannot be downgraded through normal editing.

Validation: Identity tests and vet pass, including rejected ordinary infrastructure promotion and URL boundary cases. Frontend typecheck passes; full UI suite is recorded after completion. Production callback network restrictions are enforced by the deletion worker increment, in addition to registration URI validation. Live registration/secret cutover is unverified.

Rollout: additive migration, rebuild Identity and admin; verify registered navigation targets and ordinary client creation. Existing IDs/secrets remain. Rollback: revert code and retain unused URL columns. Existing legacy auth/signup fields remain for compatibility; no Strong gate is removed.

## PR-03

Migration 016 stores versioned authentication proof and durable hashed MFA challenges, and invalidates legacy TOTP-only sessions. Combined password/TOTP requires a verified password, browser-CSRF binding, expiry, persistent five-attempt budget, current password fingerprint, replay-resistant OTP and one-use challenge consumption. No session is issued after password verification alone. Session insertion atomically rejects disabled/deleting/recovery-pending users. The portal supplies both factors with the bound server challenge; email+OTP alone fails. Existing app Strong gates stay enabled and now reject TOTP-only.

Validation: Identity tests/vet pass. A real SQLite/HTTP test enrolls encrypted TOTP, generates a real HMAC OTP, rejects passwordless and cross-browser attempts, completes MFA, verifies Strong but not WebAuthn proof, and rejects replay. Store tests cover durable attempt exhaustion, expiry and concurrent one-use consumption. Satisfaction matrix includes recovery, missing proof, unknown ACR, password, MFA and UV passkey. Hardware/browser smoke remains unverified.

Rollout: migrate schema, rebuild provider and portal assets together, require legacy TOTP users to sign in with password+TOTP or passkey. Old proof is never relabeled; v1 assurance requires reauthentication. Rollback must retain the TOTP-only rejection and challenge migration; old binaries would restore the forbidden TOTP-only Strong route. Preserve manual recovery/passkey enrollment. OIDC propagation is delivered in PR-04.

## PR-04

Signed ID tokens now carry achieved ACR, actual methods and original authentication time. Versioned proof and requested assurance/freshness survive sessions, one-time codes, access tokens and delegated tokens. Migration 017 invalidates unfinished legacy codes without trustworthy proof. Unknown ACRs and weaker/stale proof fail closed. Discovery advertises the implemented satisfaction matrix. BFF migration 006 binds step-up to the initiating subject and required WebAuthn ACR and validates both; privileged checks remain WebAuthn-only. Legacy app Strong is retired only with the explicit audited operator command documented in the evidence guide.

Validation: real SQLite/PKCE/signing round-trip test verifies RSA signature, signed ACR/AMR/nonce and unchanged auth_time for password, MFA and passkey, rejects replay, unsupported ACR, password/TOTP-only Strong and stale proof. Identity/BFF tests and vet pass. BFF tests reject wrong-subject, stale and weaker step-up. Hardware/browser and other consumer adaptation remain unverified.

Rollout: deploy provider proof/schema and portal together, then BFF with migration 006; expire unfinished legacy authorizations. Audit Strong consumers before individual gate retirement. Rollback retains proof columns, invalidates unfinished transactions and preserves TOTP-only rejection; do not return consumers to a provider that cannot satisfy required ACR. No external consumer repository was changed.

## PR-05

Authorized applications now explicitly represent active consent, not known app accounts or app roles. Bilingual revocation wording preserves app data, mappings and independent sessions. Registered management/deletion navigation is shown without appended subjects/tokens; those links confer no action authority. Consent fencing and historical deletion discovery are retained; no app-local unlink/delete API or logout notification was added.

Validation: existing real-provider HTTP re-consent, store revocation/fencing and both-language rendering/catalog tests pass. Local profile preservation and safe app-local unlink remain consumer-owned integration checkpoints, unverified here.

Rollout: rebuild provider portal and verify registered static navigation URLs. Rollback: revert wording/navigation display with additive URL fields retained; retain non-destructive revocation and signed-reason validation in every receiver. No schema migration in this increment.

## PR-06

Migration 018 adds durable deletion operations/deliveries and prevents re-enabling deleting identities. Initiation atomically freezes identity credentials and snapshots historical candidates; a bounded lease-based worker survives crashes and lost acknowledgements, reuses event IDs and refreshes signatures. Definitive cleanup, no-current-mapping, infrastructure exemption, queued, missing callback, failed and unresolved outcomes remain distinct. A scoped hashed-cookie capability resumes browser status after normal sessions end; authorized operators can inspect, retry, refresh reviewed snapshots or reconcile legacy requests. Legacy migration emits no fabricated callbacks. Recovery preparation and verification include the new state; maintenance retains outstanding work and bounds completed retention. Production delivery uses HTTPS and public-address checks at dial time with no redirects/proxies.

Validation: Identity tests/vet pass. Real-signature callback tests cover lost acknowledgement, restarted service, rotated signing key, stable IDs, atomic credential freeze, forbidden re-enable, stale session insertion, duplicate requests and user-cascade independence. Further tests cover expired leases/stale workers, crash before finalization, 202, changed callback plus explicit refresh, no-current-mapping, missing legacy callbacks, atomic rollback, outbox backup/restore, maintenance retention and legacy reconciliation without replay. Public status tests verify cookie scope, no URL secret, post-session access and unauthorized denial. Production/hardware/cross-consumer gates remain unverified.

Rollout: deploy additive schema with `IDENTITY_DURABLE_DELETION_ENABLED=false`; verify actual receiver acknowledgement/idempotency behavior and backup/recovery; then enable per deployment and observe private queue metrics. The flag is mapped by all Compose variants. Pending operations never fall back to synchronous processing when the worker is disabled. Rollback: disable dispatch, preserve schema/outbox and disabled identities; never run an old binary against outstanding operations. See the durable deletion runbook for exact retry/legacy-reconciliation steps and retention.

## PR-07

Aligned the integration guide and recovery/rollout references with actual staged behavior and appended current A01-A28 evidence. The requested full-diff review covered all increments against the original baseline and corrected normalization, timestamp precision, production URI enforcement, method/proof consistency, redirect categorization, audit atomicity, inconsistent finalization guards and misleading migration/setup UI. Migration 019 preserves authentication instant precision; BFF migration remains 006. Normal operation does not infer app accounts or roles, and no consumer repository was changed.

Validation: full Identity/BFF tests and vet, frontend typecheck/lint/142 tests/build pass. Existing Step 13 sanity/e2e scripts were repaired and an isolated Docker API/operations rehearsal passed with disposable keyrings/volumes. Frozen contract digest remains the baseline digest. See the separate review and compliance records for exact evidence and unverified gates; no production compliance or hardware certification is claimed.

Rollout: review the client inventory, require consumer ACR/freshness/subject/action checks, verify receivers and backup/recovery, then perform individual access/assurance cutovers and explicitly enable durable dispatch. TOTP credential management retains its existing passkey requirement. Old WOI lifecycle receipts are a consumer retirement checkpoint without replay. Do not retire compatibility columns until the reviewed rollback window ends.

Rollback: keep added schema and queued operations; disable worker dispatch before service rollback, retain identity freeze and receiver non-destructive disconnect safeguards, invalidate unsupported in-progress authentication state and preserve the verified passkey gates. Full live migration, browser/hardware, cross-app erasure/ownership, destructive deployment restoration and production sign-off remain explicit release blockers.

## Exact changed paths by increment

Paths are recorded for review; migrations and validation/rollback details are in each increment above.

<details>
<summary>docs(identity): record federation v1 contract baseline (PR-00) (8867166)</summary>

- `docs/MW-Identity-application-integration.md`
- `docs/dev_docs/MW-Identity-Federation-v1-Compliance.md`
- `docs/dev_docs/MW-Identity-Federation-v1-Implementation-Plan.md`
- `docs/dev_docs/MW-Identity-Federation-v1-Inventory.md`

</details>

<details>
<summary>feat(identity): stage v1 access policy and global registration (PR-01) (abbba6a)</summary>

- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `mw-identity/Dockerfile`
- `mw-identity/cmd/federation-cutover/main.go`
- `mw-identity/internal/applications/model.go`
- `mw-identity/internal/applications/service.go`
- `mw-identity/internal/auth/registration.go`
- `mw-identity/internal/auth/store.go`
- `mw-identity/internal/httpapi/identity_auth.go`
- `mw-identity/internal/httpapi/registration_test.go`
- `mw-identity/internal/oidc/admin_access.go`
- `mw-identity/internal/store/sqlite/applications.go`
- `mw-identity/internal/store/sqlite/consent.go`
- `mw-identity/internal/store/sqlite/federation.go`
- `mw-identity/internal/store/sqlite/federation_test.go`
- `mw-identity/internal/store/sqlite/oidc.go`
- `mw-identity/internal/store/sqlite/registration.go`
- `mw-identity/migrations/014_federation_access.sql`
- `mw-identity/web/static/index.html`
- `mw-identity/web/static/indexEN.html`
- `mw-identity/web/static/render.go`

</details>

<details>
<summary>feat(identity): simplify registry and isolate infrastructure provisioning (PR-02) (5ddb69d)</summary>

- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `mw-identity/cmd/create-application/main.go`
- `mw-identity/internal/applications/model.go`
- `mw-identity/internal/applications/navigation.go`
- `mw-identity/internal/applications/navigation_test.go`
- `mw-identity/internal/applications/service.go`
- `mw-identity/internal/httpapi/identity_admin.go`
- `mw-identity/internal/identityadmin/model.go`
- `mw-identity/internal/identityadmin/service.go`
- `mw-identity/internal/store/sqlite/applications.go`
- `mw-identity/internal/store/sqlite/applications_test.go`
- `mw-identity/internal/store/sqlite/identity_admin.go`
- `mw-identity/migrations/015_application_navigation.sql`
- `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx`
- `mw-web-admin/src/pages/Control/ResourcePage.tsx`

</details>

<details>
<summary>feat(identity): authenticate with bound password and TOTP proof (PR-03) (1575ed6)</summary>

- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `docs/dev_docs/federation-authentication-evidence.md`
- `mw-identity/internal/auth/assurance.go`
- `mw-identity/internal/auth/assurance_test.go`
- `mw-identity/internal/auth/mfa.go`
- `mw-identity/internal/auth/service.go`
- `mw-identity/internal/auth/store.go`
- `mw-identity/internal/httpapi/identity_strong.go`
- `mw-identity/internal/httpapi/mfa_test.go`
- `mw-identity/internal/httpapi/strong_assurance.go`
- `mw-identity/internal/store/sqlite/identity_auth.go`
- `mw-identity/internal/store/sqlite/mfa.go`
- `mw-identity/internal/store/sqlite/mfa_test.go`
- `mw-identity/internal/store/sqlite/operations.go`
- `mw-identity/migrations/016_authentication_proof.sql`
- `mw-identity/web/static/index.html`
- `mw-identity/web/static/indexEN.html`
- `mw-identity/web/static/portal.js`

</details>

<details>
<summary>feat(identity): issue signed per-action assurance and bind BFF step-up (PR-04) (bac27a3)</summary>

- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `docs/dev_docs/federation-authentication-evidence.md`
- `mw-bff/internal/httpapi/auth.go`
- `mw-bff/internal/identityclient/assurance_test.go`
- `mw-bff/internal/identityclient/client.go`
- `mw-bff/internal/identityclient/jwt.go`
- `mw-bff/internal/identityclient/model.go`
- `mw-bff/internal/store/sqlite/admin_auth.go`
- `mw-bff/migrations/006_step_up_binding.sql`
- `mw-identity/cmd/federation-cutover/main.go`
- `mw-identity/internal/auth/service.go`
- `mw-identity/internal/httpapi/consent.go`
- `mw-identity/internal/httpapi/oauth.go`
- `mw-identity/internal/oidc/delegation.go`
- `mw-identity/internal/oidc/discovery.go`
- `mw-identity/internal/oidc/model.go`
- `mw-identity/internal/oidc/service.go`
- `mw-identity/internal/store/sqlite/assurance_test.go`
- `mw-identity/internal/store/sqlite/federation.go`
- `mw-identity/internal/store/sqlite/oidc.go`
- `mw-identity/migrations/017_authorization_assurance.sql`

</details>

<details>
<summary>feat(identity): clarify authorized apps and account navigation (PR-05) (d2a069e)</summary>

- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `mw-identity/internal/auth/store.go`
- `mw-identity/internal/store/sqlite/account_lifecycle.go`
- `mw-identity/web/static/account.html`
- `mw-identity/web/static/messages.go`
- `mw-identity/web/static/messages_test.go`

</details>

<details>
<summary>feat(identity): persist and recover whole-account deletion (PR-06) (d5ebf2a)</summary>

- `.env.example`
- `docker-compose-dev.yml`
- `docker-compose.stackport.yml`
- `docker-compose.yml`
- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `docs/dev_docs/federation-durable-deletion.md`
- `infra/gateway/nginx.conf`
- `mw-identity/cmd/server/main.go`
- `mw-identity/cmd/verify-db/main.go`
- `mw-identity/internal/auth/account_lifecycle.go`
- `mw-identity/internal/auth/deletion_model.go`
- `mw-identity/internal/auth/deletion_transport.go`
- `mw-identity/internal/auth/deletion_transport_test.go`
- `mw-identity/internal/auth/deletion_worker.go`
- `mw-identity/internal/auth/service.go`
- `mw-identity/internal/auth/store.go`
- `mw-identity/internal/config/config.go`
- `mw-identity/internal/httpapi/deletion.go`
- `mw-identity/internal/httpapi/deletion_test.go`
- `mw-identity/internal/httpapi/identity_auth.go`
- `mw-identity/internal/httpapi/router.go`
- `mw-identity/internal/oidc/admin_access.go`
- `mw-identity/internal/recovery/database.go`
- `mw-identity/internal/store/sqlite/deletion.go`
- `mw-identity/internal/store/sqlite/deletion_test.go`
- `mw-identity/internal/store/sqlite/operations.go`
- `mw-identity/migrations/018_durable_deletion.sql`

</details>

<details>
<summary>PR-07: compatibility, integration validation and full diff review</summary>

- `.env.example`
- `docs/MW-Identity-Implementation-guide.md`
- `docs/MW-Identity-application-integration.md`
- `docs/dev_docs/MW-Identity-Federation-v1-Compliance.md`
- `docs/dev_docs/MW-Identity-Federation-v1-Delivery.md`
- `docs/dev_docs/MW-Identity-Federation-v1-Review.md`
- `docs/dev_docs/federated-signin-rollout.md`
- `docs/dev_docs/federation-authentication-evidence.md`
- `docs/dev_docs/runbook-disaster-recovery.md`
- `docs/dev_docs/runbook-identity-recovery.md`
- `mw-bff/internal/store/sqlite/step_up_migration_test.go`
- `mw-bff/migrations/006_step_up_binding.sql`
- `mw-identity/cmd/create-application/main.go`
- `mw-identity/cmd/federation-cutover/main.go`
- `mw-identity/cmd/server/main.go`
- `mw-identity/internal/applications/navigation.go`
- `mw-identity/internal/applications/navigation_test.go`
- `mw-identity/internal/applications/service.go`
- `mw-identity/internal/auth/assurance.go`
- `mw-identity/internal/auth/deletion_transport.go`
- `mw-identity/internal/auth/deletion_transport_test.go`
- `mw-identity/internal/auth/deletion_worker.go`
- `mw-identity/internal/config/config.go`
- `mw-identity/internal/config/config_test.go`
- `mw-identity/internal/httpapi/deletion.go`
- `mw-identity/internal/httpapi/freshness_test.go`
- `mw-identity/internal/httpapi/identity_auth.go`
- `mw-identity/internal/httpapi/identity_strong.go`
- `mw-identity/internal/httpapi/oauth.go`
- `mw-identity/internal/identityadmin/service.go`
- `mw-identity/internal/oidc/admin_access.go`
- `mw-identity/internal/oidc/service.go`
- `mw-identity/internal/store/sqlite/applications.go`
- `mw-identity/internal/store/sqlite/applications_test.go`
- `mw-identity/internal/store/sqlite/assurance_test.go`
- `mw-identity/internal/store/sqlite/deletion.go`
- `mw-identity/internal/store/sqlite/deletion_test.go`
- `mw-identity/internal/store/sqlite/federation.go`
- `mw-identity/internal/store/sqlite/federation_test.go`
- `mw-identity/internal/store/sqlite/identity_auth.go`
- `mw-identity/migrations/019_authentication_instant.sql`
- `mw-identity/web/static/account.html`
- `mw-identity/web/static/messages.go`
- `mw-identity/web/static/render.go`
- `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx`
- `scripts/e2e-step-13.ps1`
- `scripts/sanity-step-13.ps1`

</details>
