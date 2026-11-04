# MW Identity Federation v1.0 — Codex Implementation Plan

> Historical implementation/planning snapshot. The [2026-11-04 clean v1 migration decision](federation-v1-clean-migration.md) supersedes compatibility, application-wide Strong and per-application signup behavior described below.

> **Status:** Implementation roadmap; no production behavior changes authorized by this document
> **Repository:** `vtmattedi/mw-plane-control`
> **Date:** 2026-11-03
> **Normative input:** `MW-Identity-Federation-Contract-v1.0.md` (provided with this plan). The contract takes precedence over this roadmap and legacy behavior.
> **Delivery:** Small, sequential PRs. Do not implement the entire plan in one change.

## 0. Codex execution rules

1. Read the **complete frozen contract** before editing. If it has not yet been added to the repository, add or link the supplied document as a first documentation-only change. Do not improvise contract semantics.
2. Inspect the current default branch; record the HEAD SHA, outstanding modifications, actual migration version, and current tests. File paths below are verified starting points from the 2026-11-03 review, not a substitute for repo inspection.
3. Implement one PR increment at a time; stop after each increment and report the exact changed paths, tests, migration/compatibility risk, rollout and rollback steps, and any unresolved contract issues.
4. Do not change WOI, Stackport, or other application repositories in this plan. Record consumer obligations as integration checkpoints.
5. Protect current OIDC Authorization Code + S256 PKCE, consent-version checks, JWT/JWKS validation, existing MW BFF WebAuthn step-up, administrator authorization, and destructive-callback validation throughout migration.
6. **Fail closed** for wrong/missing assurance, stale authentication, changed private eligibility, revoked consent, deleting/disabled identity, invalid issuer/audience/signature, and unsupported ACR.
7. Prefer additive SQLite migrations; never drop production columns/records or bulk-replay historical lifecycle receipts as part of a new contract cutover.
8. Never equate historical consent with an existing application account. No central association registry or generation handshake is part of v1.0.
9. Run the tests available in the environment. If integration, real WebAuthn or deployment gates cannot run, explicitly label them *unverified*, not passing.
10. Don't silently make an old Strong-protected client password-accessible when removing the app-level `authentication_policy` field. Assure migrated consumer behavior before retiring that gate.

## 1. Scope and non-goals

### Goals

- **Access:** Public/private application eligibility, independent of OAuth public/confidential client type and identity registration.
- **Registration:** Globally open MW identity portal signup; remove per-client `allow_public_signup` as an authorization or registration policy.
- **Authentication:** Per-action Standard/Strong assurance via signed OIDC `acr`, truthful `amr` and real `auth_time`; request explicit step-up with `acr_values` and `max_age`.
- **Strong:** Passkey with required user verification, or a real combined password + TOTP ceremony; never TOTP-only.
- **Consent:** Revocation is non-destructive. Reauthorization returns the same stable `iss`/`sub`; apps reuse their existing local profile mappings.
- **Application registry and UX:** Simplified normal client admin, protected MW-infrastructure provisioning, optional account-management/deletion navigation URLs, accurate user-facing consent wording.
- **Whole-identity deletion:** Atomic disable and work creation, durable callback delivery, stable event IDs, restart recovery, correct per-app outcomes and secure progress UI.
- **Assurance:** No implicit superadmin bypass of an ordinary private app's allowlist or explicit denial.

### Non-goals

No application-owned account mapping in MW Identity; no direct cross-app unlink/delete API; no synchronization of application-issued sessions; no automatic logout of local app sessions on consent revocation; no refresh-token or offline access grant; no new app-local role management; no email-based account merge; no work directly in WOI during the MW provider implementation.

## 2. Existing implementation: verified starting points

| Concern           | Current behavior / issue                                                                                                                                                           | Code locations                                                                                                                                                      |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Client registry   | `application_type=confidential/public`; `authentication_policy=standard/strong`; `access_policy=default_allow/default_deny`; `allow_public_signup`; `internal`; callback | `mw-identity/internal/applications/{model,service}.go`, `internal/identityadmin/{model,service}.go`, `internal/store/sqlite/{applications,identity_admin}.go` |
| Access            | `allow_public_signup=1` can admit an unassigned user despite `default_deny`. A global superadmin may bypass normal access rules.                                               | `mw-identity/internal/oidc/admin_access.go`, `internal/store/sqlite/{consent,oidc}.go`                                                                          |
| Registration      | Portal standalone signup exists, but app-initiated registration depends on per-app signup flag.                                                                                    | `mw-identity/internal/auth/registration.go`, `internal/store/sqlite/registration.go`, `internal/httpapi/identity_auth.go`                                     |
| Strong            | App-wide Strong accepts WebAuthn or TOTP-only. Existing WebAuthn-specific `acr_values=urn:mw:webauthn` and `max_age` already exist.                                            | `mw-identity/internal/httpapi/{oauth,identity_strong,strong_assurance}.go`, `internal/identitystrong/service.go`                                                |
| Assurance claims  | Signed ID tokens have `auth_time` and `amr` but no `acr`; TOTP normalization can become `unknown`.                                                                         | `mw-identity/internal/oidc/service.go`                                                                                                                            |
| Consent           | Active consent distinct from assignments; revocation removes MW tokens/codes, not local app sessions; code/token version checks.                                                   | `mw-identity/internal/store/sqlite/consent.go`, `internal/httpapi/consent.go`, `internal/store/sqlite/oidc.go`                                                |
| Deletion          | Disables identity and calls apps within the HTTP request, with up to three attempts. No resumable outbox.                                                                          | `mw-identity/internal/auth/account_lifecycle.go`, `internal/store/sqlite/account_lifecycle.go`, `cmd/server/main.go`                                          |
| Admin + portal UX | App-wide policy, signup, and internal switches exposed in app form; consent is labeled as connected applications.                                                                  | `mw-web-admin/src/pages/Control/{ResourcePage,IdentityDetailPages}.tsx`, `mw-identity/internal/httpapi/identity_auth.go`, `mw-identity/web/static/`           |
| Legacy schema     | Existing `013_application_consent.sql` (as reviewed); find newer migration at execution time.                                                                                    | `mw-identity/migrations/`                                                                                                                                         |

### Critical risks

1. Eligibility rules are repeated in Go service logic **and SQL token/consent predicates**. Update both atomically; changing only one can leave a bypass.
2. Existing `default_deny + allow_public_signup=1` applications may be effectively public. Inventory and remap their intended policy before removing the bypass, or risk lockouts.
3. Retiring app-wide Strong prematurely can downgrade MW admin/BFF or other protected clients. Preserve WebAuthn-only admin operations.
4. The existing TOTP login is a *single-factor* email+TOTP route; relabeling its tokens as Strong is forbidden.
5. Historical consent/assignments disappear with the identity row (foreign-key cascades). Pending deletion deliveries must survive independently.
6. Existing deletion callback receivers may treat any `2xx` as success. Do not treat asynchronous `202` as proof of data erasure.

## 3. PR-00 — Baseline, frozen contract and conformance map

**Purpose:** Establish authoritative documentation and measurable starting point. **No functional changes.**

**Tasks**

- [ ] Check in `docs/MW-Identity-Federation-Contract-v1.0.md` from the supplied freeze file, preserving exact text.
- [ ] Check in this plan at `docs/dev_docs/MW-Identity-Federation-v1-Implementation-Plan.md`.
- [ ] Add `docs/dev_docs/MW-Identity-Federation-v1-Compliance.md` mapping contract scenarios **A01–A28** to implementation/test evidence and status (`pass`, `fail`, `unimplemented`, `unverified`).
- [ ] Inventory every read/write of `authentication_policy`, `allow_public_signup`, `internal`, `default_allow/default_deny`, `amr`, `auth_time`, `acr_values`, `delete_callback_uri`, and consent/deletion APIs across Go, TS, CLI, scripts and docs.
- [ ] Audit current registered clients and intended public/private access; record clients relying on signup bypass, global superadmin exception or app-wide Strong.
- [ ] Run baseline tests/build and existing Step 13 checks, recording failures before edits. Verify SQLite backup/restore procedure with disposable data.
- [ ] Update legacy integration-guide introduction to distinguish current deployed behavior from contract target; do not claim v1 compliance.

**Gate:** Baseline and migration inventory exist; no user-visible/authentication behavior changed.

## 4. PR-01 — Public/private access and global signup

**Purpose:** Make public/private the only normal app eligibility dimension. Signup is a global identity capability, not a client permission.

**Tasks**

- [ ] Preserve existing storage mapping `default_allow`→Public, `default_deny`→Private; normalize API/UI naming without changing existing client IDs.
- [ ] Centralize access semantics or introduce one shared, tested policy representation used by `internal/oidc/admin_access.go`, token issuance and the SQL predicates in `internal/store/sqlite/{consent,oidc}.go`.
- [ ] Ensure explicit `deny` wins on Public apps, explicit `allow` required on Private apps, and no regular superadmin bypass. Preserve separate MW Identity admin-audience restrictions.
- [ ] Remove the `allow_public_signup` condition from authorization eligibility and credential re-checks. **Before activation**, audit every live `default_deny + allow_public_signup=1` client; intentionally migrate it to Public or preserve Private with explicit assignments. Never guess.
- [ ] Remove the app-specific registration dependency in `internal/auth/registration.go`, `internal/store/sqlite/registration.go`, and `internal/httpapi/identity_auth.go`. Maintain standalone MW portal registration; app-initiated registration cannot create app eligibility, consent or local roles. Honor the global registration policy (a global emergency disable is distinct from an app flag).
- [ ] On private-app login UX, hide/de-emphasize signup invitation and explain restricted application access. Do not block direct standalone registration. Keep PT-BR and EN equivalent.
- [ ] Revalidate eligibility at authorization completion and **code-to-token issuance**; protect against mid-flow assignment removal, explicit deny, consent revoke, app disable and identity deletion. Avoid TOCTOU gaps in SQL transactions.
- [ ] Keep legacy schema columns during transition; mark deprecated only after migrated callers stop writing them.

**Acceptance:** A01–A06, A21–A22; existing public identities work in Public apps; unassigned users cannot access Private apps despite prior consent; explicit denies work; superadmins do not bypass ordinary-app assignments; no application-local role is issued; empty/populated migration fixtures succeed.

**Rollout gate:** Review old `default_deny + allow_public_signup=1` population before enforcing the new policy. No silent access broadening or lockout.

## 5. PR-02 — Client registry, secure infrastructure provisioning, navigation metadata

**Purpose:** Simplify ordinary application admin and add app account-management navigation without remote account control.

**Tasks**

- [ ] Ordinary app create/edit: name, Public/Private access, OAuth Confidential/Public type, exact redirects/post-logout redirects, allowed scopes/claims/audiences, required `delete_callback_uri` for apps with user data, optional `account_management_uri` and `account_deletion_uri`, enabled/archived status.
- [ ] Remove `authentication_policy`, `allow_public_signup`, and `internal` toggles from the *normal UI*. Preserve old API/schema fields for compatibility until later retirement. Changes to deprecated fields must not silently alter policy.
- [ ] Add additive SQLite columns for navigation URLs using the actual next numbered migration. Update `internal/applications/{model,service}.go`, `internal/identityadmin/{model,service}.go`, `internal/store/sqlite/{applications,identity_admin}.go`, `internal/httpapi/identity_admin.go`, and `mw-web-admin/src/pages/Control/ResourcePage.tsx`.
- [ ] Validate navigation URLs as absolute HTTPS, bounded length, no credentials/fragments. No user identity or token appended. Preserve test/development exception rules explicitly. For server-to-server callback URLs, enforce SSRF-safe endpoint restrictions appropriate to production.
- [ ] Ensure only Confidential clients get a generated, hashed, server-held client secret; Public OAuth clients get no secret and use PKCE. Test rotation and failure cases.
- [ ] Move MW `internal` clients into a distinct narrowly controlled and audited infrastructure provisioning route/CLI/bootstrap. Do not silently promote ordinary Private clients into this class. Preserve existing trusted infra clients.
- [ ] Preserve callback requirement for ordinary clients; detect and report legacy callback-less clients as unresolved.
- [ ] Update normal admin app form and details (PT-BR and EN), backend API tests and migration tests.

**Acceptance:** A26–A28. Existing clients continue using original IDs/secrets. App Public/Private cannot be confused with OAuth Public/Confidential. New navigation metadata survives round trip. Unauthorized creation of special infrastructure clients denied.

## 6. PR-03 — Trustworthy authentication evidence and real MFA

**Purpose:** Model the actual authentication event so that later OIDC assurance claims are truthful.

**Design deliverables before implementation**

- [ ] Document durable session proof representation: constituent factors, WebAuthn user-verification result, authentication instant, provenance, and what changes on session reauthentication. Preserve through sessions → authorization codes → token exchange. Decide handling of pre-v1 sessions.
- [ ] Define combined **password → TOTP** flow with bound server-side short-lived single-use challenge, expiration, CSRF, attempt and rate limits, replay protection, and session issuance only after both factors succeed. Do not store raw password/TOTP secrets in challenge state.
- [ ] Retain verified WebAuthn/passkey flows. For `urn:mw:webauthn` ensure actual user verification, not presence of a credential alone.
- [ ] Define migration for current email+TOTP-only logins/sessions. Never promote an old `auth_method=totp` session to Strong. Force safe reauthentication; keep passkey/manual recovery paths available.

**Code tasks**

- [ ] Implement combined MFA in `internal/identitystrong/service.go`, `internal/httpapi/identity_strong.go` and auth/session storage. Disable/deprecate TOTP-only direct login once alternative flows and migration are tested.
- [ ] Extend SQLite auth-session and OAuth proof fields additively as needed. Persist trustworthy proof facts and timestamps; avoid storing bearer challenges in cleartext.
- [ ] Introduce a shared assurance-evaluator function with unit tests. `standard`: password or passkey; `strong`: UV-verified passkey OR completed password+TOTP; WebAuthn-specific: UV-verified passkey only. Recovery-only sessions insufficient.
- [ ] Keep current BFF/MW-admin fresh WebAuthn requirement for credential rotation and privileged mutations.
- [ ] Implement enrollment/onboarding and fallback UX for missing passkey/TOTP; maintain both languages.

**Acceptance:** A14, A16–A18. TOTP-only never Strong; combined factors produce stable evidence; passkey UV checked; old sessions cannot be upgraded by labeling; replay, brute-force, multi-browser, expiry, recovery and concurrency tests pass. Run actual hardware/browser authentication smoke test separately from mocked tests.

**Rollout:** Ship compatible new MFA first; migrate users; reauthenticate legacy TOTP-only sessions. Do not remove old app-wide Strong gates in this PR.

## 7. PR-04 — OIDC ACR/AMR and per-operation step-up

**Purpose:** Replace global Strong app policy with action-scoped assurance using signed evidence.

**Tasks**

- [ ] Support exact requested ACRs: `urn:mw:auth:standard`, `urn:mw:auth:strong`, and existing `urn:mw:webauthn`. Unknown ACRs return explicit errors; no silent downgrade.
- [ ] Make `internal/httpapi/oauth.go` enforce requested actual assurance and freshness. `max_age=0` performs new qualifying authentication; positive `max_age` evaluates the correct proof timestamp, not token mint time. Preserve current WebAuthn step-up semantics and BFF gates.
- [ ] Carry achieved assurance/factors/authentication time through authorization codes and token exchange, rejecting tampered/stale or policy-mismatched requests.
- [ ] Emit truthful `acr`, `amr` and `auth_time` in OIDC ID tokens (`internal/oidc/service.go`). Use documented standard method identifiers (`pwd`, `otp`, `mfa` where applicable) and documented passkey identifier. Do not normalize valid MFA to `unknown`.
- [ ] Publish actual supported ACRs in OIDC discovery (see `internal/oidc/` and `internal/httpapi/oidc_metadata.go`). Include a formal satisfaction matrix. A passkey satisfies Strong and Standard; password+TOTP satisfies Strong but **not** WebAuthn-only.
- [ ] Update MW BFF consumer parsing if needed (`mw-bff/internal/identityclient/client.go`), but do not weaken high-risk WebAuthn-only actions.
- [ ] **Stage removal of app-level `authentication_policy` enforcement only after protected clients have adopted and passed their own step-up checks.** Keep a per-client migration inventory and explicit cutover. Then retire old UI/API persistence fields after rollback window.
- [ ] Provide PT-BR/EN user flows for requested step-up and missing strong credential.

**Acceptance:** A14–A20. An app can sign in at Standard and request fresh Strong/WebAuthn for one action; correct signature/issuer/audience/subject and `auth_time` checked. Explicit fresh passkey challenge cannot be satisfied by password+TOTP. Reissued tokens do not become newly authenticated. ACR discovery matches behavior. Existing BFF high-risk gates remain intact.

**Compatibility note:** OIDC consumers need to validate newly introduced `acr` correctly; do not require WOI changes in this provider PR, but provide a disposable client fixture and consumer migration guide.

## 8. PR-05 — Consent, account navigation and truthful portal UX

**Purpose:** Preserve strict separation of consent from app account association and deletion.

**Tasks**

- [ ] Keep consent-version binding and atomic revocation of outstanding MW access/delegated tokens and authorization codes.
- [ ] Rename *Connected applications* to **Authorized applications** in Identity portal (PT-BR and EN). Explain that consent does not prove local profile exists.
- [ ] Rename remove/disconnect action to **Revoke MW access**, with accurate warning that app-owned profile, roles/data and independent sessions survive.
- [ ] Display registered external navigation for **Manage application account** and **Delete account in application**, when provided. Links grant no deletion authority and embed no `sub`/token.
- [ ] Preserve historic application list separately if helpful, with no false claim about current profiles.
- [ ] Verify re-consent after revocation; issuing the same `iss`/`sub` for the same MW identity. Verify private-app eligibility is rechecked at reauthorization.
- [ ] No local unlink or app deletion API is added to MW Identity. Application must authenticate and confirm the target account independently.

**Acceptance:** A07–A13, A21. Revoke only the last consent leaves MW identity intact; no destructive event or app logout promised. Authorization to the same `iss`/`sub` after re-consent remains possible if eligible.

## 9. PR-06 — Durable whole-identity deletion

**Purpose:** Make whole-MW deletion correct under browser close, callback outage, process restart and retries.

**Design gate (document before code)**

- [ ] Define deletion operation state machine and durable callback outcome taxonomy: pending, delivering, confirmed cleanup, no-current-local-mapping, failed/retryable, unresolved/legacy. `202` cannot be interpreted as confirmed erasure.
- [ ] Define snapshot of **candidate applications** from historical consent and assignment only; this does **not** prove a local account exists.
- [ ] Define stable operation ID, per-app delivery event IDs, original requested time, retry semantics, key rotation behavior, event-signature claims, and minimal post-user-deletion tombstone retention.
- [ ] Design user-visible resumption after session invalidation: narrowly scoped short-lived status capability, random and stored hashed, never solely an operation ID and never logged/leaked in URL referrers. Include operator-access alternative.
- [ ] Define migration and recovery for existing `deletion_requested_at_unix` disabled users; old event IDs cannot be invented or reconstructed.

**Storage/worker**

- [ ] Add SQLite tables (illustrative names) `identity_deletion_operations` and `identity_deletion_deliveries` with operation ID, subject, timestamps, delivery state, event ID, app/client snapshot, attempts, next attempt, lease expiry/worker, safe error category and completion status. Avoid FKs cascading away pending work when user row is deleted.
- [ ] In a single transaction, disable identity, freeze authorization/enrollment, revoke MW sessions/codes/tokens, create or reuse deletion operation, snapshot delivery targets, enqueue work. **No HTTP inside transaction.**
- [ ] Refactor `internal/auth/account_lifecycle.go` to acknowledge accepted/pending deletion independent of outbound request. Keep request context only for initiating DB transaction.
- [ ] Introduce bounded, crash-recovering worker started and stopped from `mw-identity/cmd/server/main.go`. Use SQLite-safe atomic claim/lease and bounded concurrency, timeout, backoff+jitter, retry horizon and operator-visible failure state. HTTP redirects rejected, responses size-bounded, no raw bearer tokens in logs.
- [ ] Retry with **stable event ID** and operation identity, but newly signed short-lived JWT proof under current active signing key. Match `iss`, `sub`, exact client `aud`, `jti`, signed reason, issued/expiry and body/header event IDs.
- [ ] Only `identity.user.deleted` with signed `reason=account_deleted` may trigger destructive processing. Consumers must ignore legacy `application_disconnected` non-destructively. An existing local unlink must protect independent profile from delayed callbacks.
- [ ] Define definitive success acknowledgement and optional differentiated no-current-mapping response. Do not count `202` as completed cleanup; if changing receiver response protocol, version it and coordinate with consumers.
- [ ] Permit operator status/retry with proper MW admin authorization/audit; show pending/failed/completed truthfully to user through scoped status capability. Final central user removal only when required outcomes satisfy policy, retaining minimal operational/replay data.
- [ ] Back up and restore pending work; stop/restart worker gracefully; do not allow admin re-enable or recovery to reopen an identity during deletion.

**Acceptance:** A23–A25. Crash before worker start, crash after app acknowledges but before DB commit, callback timeout and return to service, duplicate claims, signature rotation, callback URL change, repeated user requests, stale auth transactions and unknown legacy apps are all explicitly tested. No lost jobs; no guessed deleted accounts; no silent success for offline app.

**Staged rollout:** Deploy schema with worker off; verify app callback compatibility and disposable e2e; enable worker in controlled environment; reconcile previously disabled identities; verify production backup and recovery; enable with queue-age/failure observability.

## 10. PR-07 — Final compatibility, integration and release gate

**Tasks**

- [ ] Align `docs/MW-Identity-application-integration.md`, API/discovery docs, admin usage, PT-BR/EN portal text, operations/recovery/backup documents and release notes with deployed behavior.
- [ ] Verify old client IDs and secrets, confidential/public token exchange, redirect/PKCE rules, and JWKS key rotation. No downgrade of admin WebAuthn gates.
- [ ] Confirm all clients historically relying on app-wide Strong or signup-based access are migrated; remove obsolete fields only after documented rollback and schema window.
- [ ] Coordinate WOI **in its own repository**: step-up validates `acr`/`amr`/`auth_time`, subject and local action intent; account deletion/linking stays local; consent revoke remains non-destructive; callback idempotency and no-current-link outcomes correct; unused `awaiting_contract` operations are retired **without replay**.
- [ ] Run isolated cross-service tests with disposable SQLite/PostgreSQL and uploads, real signatures, deliberate callback outages, and browser WebAuthn/TOTP flows.
- [ ] Record A01–A28 conformance evidence and signed-off production rollout/rollback.

### Required commands (from each module)

```bash
cd mw-identity
go test ./...
go vet ./...

cd ../mw-bff
go test ./...
go vet ./...

cd ../mw-web-admin
npm ci
npm run typecheck
npm run lint
npm run test:run
npm run build
```

Also run relevant existing `scripts/sanity-step-*.ps1` and `scripts/e2e-step-*.ps1`, including Step 12 strong-auth and Step 13 readiness, from the repo's supported environment. Do not claim to have executed them unless actually run.

### Release blockers

- Any A01–A28 scenario failing or unverified without documented exception.
- Strong can be asserted with TOTP-only or unknown proof.
- Private access bypass from signup flag, consent, or ordinary superadmin role.
- App-wide Strong gate removed before protected consumers can request/check step-up.
- Consent revocation triggers destructive app callback.
- Deletion job lost after reboot, prematurely acknowledged, or finalized while required app is offline.
- Pending deletion records cascade with deleted MW user, or migrations break recovery/restore.
- Inaccurate operator/user erasure status, unverified backup, or undocumented rollout/rollback plan.

## 11. Proposed PR sequence

| PR    | Implementation                                               | Dependencies                   | Acceptance coverage |
| ----- | ------------------------------------------------------------ | ------------------------------ | ------------------- |
| PR-00 | Contract, baseline, dependency inventory, compliance map     | None                           | Coverage map        |
| PR-01 | Public/private access + global signup                        | PR-00 + legacy app audit       | A01–A06, A21–A22  |
| PR-02 | Registry simplification, infrastructure separation, app URLs | PR-00; preferably PR-01        | A26–A28            |
| PR-03 | Password+TOTP MFA and truthful proof model                   | PR-00                          | A14, A16–A18       |
| PR-04 | ACR/AMR claims and per-action step-up                        | PR-03 + consumer compatibility | A14–A20            |
| PR-05 | Consent navigation and portal wording                        | PR-02                          | A07–A13, A21       |
| PR-06 | Durable whole-account deletion                               | PR-00 + callback compatibility | A23–A25            |
| PR-07 | Full integration and release validation                      | All                            | A01–A28            |

PR-03/PR-04 and PR-06 can be developed in independent branches after PR-00, but production capability must not be enabled until all required consumer and migration gates pass. Avoid merging all PRs as one large change.

**Stop condition:** When a planned change would unintentionally weaken a security guarantee or break an unknown consumer, stop that increment and document a staged compatibility solution. Do not silently rewrite the frozen contract.
