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
