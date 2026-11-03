# Federation v1 conformance and baseline

PR-00 baseline: 2026-11-03, `main` and local `origin/main` at `3bf0ca4f53139568f711160a7c269843557708d7`. Remote was not fetched. Working tree was clean. Identity's latest migration is `013_application_consent.sql`; no migration is introduced by PR-00. The frozen contract was already tracked and is preserved byte-for-byte (SHA-256 `df895a4e3084d7988cbdb9e2a08a47a090bdcf259f6bf05b22a6ff760c7fa7ff`).

The user authorized sequential implementation with one commit per PR increment. This overrides the supplied plan's instruction to stop after every increment. Production activation still requires its compatibility and verification gates. No consumer repository is modified.

`pass` means the named scope is demonstrated by the cited evidence; `fail` means existing code contradicts the target; `unimplemented` means required provider behavior is absent; `unverified` means runtime or consumer evidence is missing. Passing existing tests is not full contract conformance.

## Baseline acceptance map

Paths below are relative to `mw-identity/` unless qualified. Consumer-owned behavior remains unverified in this provider repository.

| ID | Status | Baseline evidence / remaining work | Increment |
| --- | --- | --- | --- |
| A01 | pass | `internal/httpapi/registration_test.go`: `TestStandaloneRegistrationCreatesUnconnectedPublicAccount`, global emergency switch tested | 01 regression |
| A02 | unverified | `internal/oidc/admin_access_test.go` permits default-allow; full target flow and app-owned onboarding not demonstrated | 01,07 |
| A03 | fail | `internal/oidc/admin_access.go` and SQL consent predicate admit default-deny + signup without assignment | 01 |
| A04 | unverified | Assignment allow is unit-tested; no end-to-end proof of app-local role preservation | 01,07 |
| A05 | fail | Deny works for ordinary identities, but global superadmins bypass it | 01 |
| A06 | fail | `AuthorizeIdentityUser` and SQL eligibility explicitly bypass for superadmins | 01 |
| A07 | unverified | Stable subject supplied; profile/data/roles belong to consumer and require cross-service tests | 07 |
| A08 | unverified | Consent is stored without account registry, but portal terminology suggests connected accounts | 05,07 |
| A09 | pass | `internal/store/sqlite/account_lifecycle_test.go`: revoke removes only app credentials, preserves assignments; no destructive callback | 05 regression |
| A10 | unverified | `TestPublicConsentDisconnectReconsentAndRecentAuthentication` demonstrates re-consent; same local profile requires consumer proof | 05,07 |
| A11 | unverified | Application-owned alternative-login safeguard | 07 consumer |
| A12 | unverified | Application-owned unlink/re-link proof and detached-account protection | 07 consumer |
| A13 | unimplemented | No registered management/deletion navigation metadata | 02,05 |
| A14 | fail | Signed `auth_time` exists but ID token lacks `acr`; password AMR uses legacy method name | 03,04 |
| A15 | unimplemented | `urn:mw:auth:strong` is not supported | 03,04 |
| A16 | unimplemented | Current TOTP login checks email + OTP, not a combined password + OTP challenge | 03,04 |
| A17 | fail | `strongSessionMethod` accepts TOTP-only | 03,04 |
| A18 | unverified | `TestBeginRegistrationAndDiscoverableLoginPolicy` requires UV; no v1 ACR or real hardware/browser smoke evidence | 03,04,07 |
| A19 | unverified | Explicit WebAuthn and fresh-login logic exist; hardware and consumer subject/action binding not verified | 04,07 |
| A20 | unverified | BFF fresh-WebAuthn tests pass; cross-service wrong-subject action-intent checks not verified | 04,07 |
| A21 | pass | `TestConsentFencesInFlightCredentialsAndPreservesHistory` rejects stale form/code/token versions | 01,05 regression |
| A22 | fail | SQL fences normal assignment denial but signup/superadmin bypasses violate private removal semantics | 01 |
| A23 | unimplemented | `auth/account_lifecycle.go` makes three request-bound attempts; no persistent deletion operation/worker | 06 |
| A24 | unverified | Retry event ID stable within a request only; durable retry absent; receiver dedup/detached protection needs consumer tests | 06,07 |
| A25 | unverified | Disabled-user checks exist; durable deletion/enrollment races and application-local handoff not verified | 06,07 |
| A26 | unverified | Public create response omits secret and real PKCE exchange tested; full protocol rejection matrix requires expansion | 02,07 |
| A27 | unverified | Confidential secret creation/verifier tested; actual fresh-WebAuthn rotation and consumer cutover not rehearsed | 02,07 |
| A28 | fail | Normal admin application payload and UI allow `internal` creation | 02 |

## Baseline checks

Environment: Windows amd64, Go 1.26.5, Node 24.14.0, npm 11.9.0, Docker server 28.4.0.

| Command/check | Result | Evidence / limit |
| --- | --- | --- |
| Identity `go test ./...` | pass | All packages pass (cached results where available) |
| Identity `go vet ./...` | pass | Exit 0 |
| BFF `go test ./...` | pass | All packages pass (cached results where available) |
| BFF `go vet ./...` | pass | Exit 0 |
| Identity `go test -count=1 -v ./internal/recovery -run '^TestEncryptedBackupAndPreparePreserveRootAndInvalidateSession$'` | pass | Uncached temporary SQLite fixture; encrypted backup, validation and isolated restore preparation preserve identities/assignments, clear sessions, retain restrictions and leave source unchanged |
| `pwsh -NoProfile -File scripts/sanity-step-13.ps1 -EnvFile .env` | fail | Go checks and Compose validation complete; script references missing `docs/runbook-disaster-recovery.md` after docs moved to `docs/dev_docs/` |
| `pwsh -NoProfile -File scripts/e2e-step-13.ps1` | unverified | Exits 1: missing disposable `.env.step12.local`; no stack started or mutated |
| Frontend `npm ci` | pass | Lockfile unchanged; reports 14 existing dependency advisories (8 moderate, 6 high); no dependency upgrades in PR-00 |
| Frontend `npm run typecheck` | pass | Exit 0 |
| Frontend `npm run lint` | pass | Exit 0; existing exhaustive-deps warning at `src/pages/Projects/Projects.tsx:199` |
| Frontend `npm run test:run` | pass | 26 files, 142 tests |
| Frontend `npm run build` | pass | Exit 0; existing large-chunk warning |
| Real WebAuthn, client-secret cutover, full destructive deployment restore and manual recovery | unverified | No isolated configured running Identity/BFF stack or hardware ceremony used |

The recovery test demonstrates isolated recovery preparation, not live service replacement or the complete Compose restore script. Do not present it as a production restore rehearsal.

## Client migration inventory and activation gates

No local Identity database, running or stopped `mw-identity` container was found. Production registration rows and owners' intended access policies were not available; the live client audit is **unverified**, not an empty client population. Do not infer intentions from example bootstrap scripts or current consent history.

Before activation, an authorized operator must take a verified backup and export only this non-secret inventory from the actual database:

```sql
SELECT id, client_id, access_policy, allow_public_signup,
       authentication_policy, internal, enabled, archived_at_unix,
       CASE WHEN COALESCE(delete_callback_uri, '') = '' THEN 0 ELSE 1 END AS has_delete_callback
FROM identity_applications ORDER BY client_id;
```

For each `default_deny + allow_public_signup=1` row, record owner-approved Public conversion or Private explicit allow assignments. Identify ordinary-app superadmin exceptions and the intended explicit assignments. For each Strong client, record consumer ACR/freshness enforcement and tested cutover before retiring the old gate. Preserve privileged BFF WebAuthn-only operations. Inventory infrastructure classification separately; ordinary Private is not infrastructure. Keep client IDs, secrets and consumer URLs intact.

Retain results in an access-controlled operator record; never export secret hashes or personally identifying assignment rows into public docs. Live audit, consumer cutover, real WebAuthn and production recovery remain release blockers.

## Dependencies and compatibility

See the complete [baseline dependency inventory](MW-Identity-Federation-v1-Inventory.md) for exact matching files/lines across Go, SQL, CLI, TypeScript, scripts and docs. Update Go eligibility and SQL credential predicates together. Keep deprecated registry columns for rollback; do not silently remove a Strong gate. No app-owned lifecycle API, central association registry or session notification belongs to v1.

PR-00 changes only documentation. Rollout: review these docs against the preserved contract. Rollback: revert this increment's documentation commit. Database, runtime authentication and registered clients are unaffected. No production compliance is claimed.
