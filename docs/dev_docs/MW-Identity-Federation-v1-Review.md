# Federation v1 full-diff code review

Reviewed the complete change against `3bf0ca4f53139568f711160a7c269843557708d7`, including the sequential PR-00 through PR-07 increments. Review covered authentication and BFF callbacks; Go and SQL eligibility; registration and infrastructure boundaries; migrations; worker claims, retry/finalization and transport; session/proof storage; portal/admin UI; operational configuration; tests and documentation. The normative frozen contract remains unchanged.

## Findings fixed before the final commit

| Finding | Fix | Regression evidence |
| --- | --- | --- |
| Reserved-scope/audience checks ran before normalization, allowing whitespace to evade ordinary-client privilege restrictions | Trim before checking reserved scopes/audiences; ordinary API still cannot provision infrastructure | `TestLegacyApplicationWithoutDeleteCallback` now checks whitespace-wrapped reserved registrations |
| Second-resolution session time could accept a different older session in the same second for `max_age=0` | Migration 019 preserves the precise authentication instant; compare it against request start without truncation | `TestMaxAgeZeroRejectsAnOlderDifferentSessionWithinSameSecond`; existing fresh-login/resume HTTP test still passes |
| Development localhost exceptions were not explicitly closed by production registry entry points; non-HTTPS localhost schemes also needed rejection | Explicit production HTTPS policy in registry service, admin update and host CLI; only HTTP localhost is a development exception; normalize production environment spelling | `TestProductionClosesLocalhostHTTPException`, `TestProductionModeNormalization`, URI validation tests |
| Independent method/proof fields could diverge when issuing credentials | Require method to match versioned proof at both code issuance and exchange; verify OTP subject matches challenge subject | Signed-assurance test rejects mismatched proof/method; real combined-MFA HTTP test |
| Persisted pre-upgrade BFF transactions had no subject/assurance binding and could otherwise complete as ordinary sign-in | Migration 006 invalidates unfinished legacy transactions instead of guessing their original intent | `TestStepUpMigrationRejectsUnboundLegacyTransactions` also verifies new bindings survive persistent storage |
| Redirect refusal surfaced as a generic retry instead of an unresolved callback problem | Keep redirects forbidden and record the explicit unresolved outcome without forwarding credentials | `TestDeletionRedirectIsNotFollowedOrAcknowledged` |
| Combined CLI cutovers could partially apply before a later invalid argument failed | Validate operation arguments before opening/migrating the database and require separate reviewed access and assurance invocations; audit client/policy metadata | Invalid-operation CLI smoke test confirms no database is created; access integration tests and atomic audited storage transactions |
| Deletion finalization could report completion for an inconsistent restored subject state | Refuse finalization if the subject is active or missing its deletion marker | Finalization transaction state check; atomic rollback/backup and restart tests |
| Legacy access labels and credential setup text could misrepresent current behavior | Display a legacy-migration warning, make infrastructure forms read-only, remove the ordinary internal-promotion suggestion, distinguish passkey-only setup, and explain password+TOTP/passkey enrollment requirements | Frontend typecheck/tests/build; bilingual portal catalog/render tests |
| Infrastructure creation audit was separate from its creation transaction | Record infrastructure provisioning inside the application creation transaction, in addition to the CLI audit | Registry/store suite and transaction review |
| Step 13 referenced documentation at obsolete paths | Update the sanity/e2e script references | Isolated `STEP 13 STATIC PASS` and `STEP 13 API/OPS PASS` |

## Verification and remaining limits

Identity and BFF test suites and vet pass. Frontend typecheck, lint, all 142 tests in 26 files and production build pass. Existing lint/chunk-size warnings and baseline dependency advisories are recorded in the compliance document. The isolated Docker rehearsal used separately named containers/volumes and generated test keys, without reading production secrets. It passed readiness/SQLite invariants, query-safe logging, private metrics, signing-key rotation, online backup verification, restore validation and container hardening.

Additional tests cover real RSA signatures, PKCE/code replay, consent fencing, private assignment removal before read/consume/issuance, actual password plus HMAC-TOTP, browser binding, challenge attempts/expiry/replay/concurrent consumption, lost callback acknowledgement, service restart, stale leases, key rotation, callback changes, 202, callback-less clients, retry exhaustion/recovery, legacy migration without replay, atomic rollback, outbox backup and maintenance retention, and scoped public progress access.

No unresolved code defect was identified after the fixes in this reviewed scope. This is not independent security certification or production release approval. Required outstanding release gates are:

- Production client inventory, owner-approved access cutover and protected-consumer step-up migration.
- Actual browser/hardware WebAuthn and browser password+TOTP enrollment/login rehearsals. TOTP setup/changes deliberately retain the existing recent-passkey guard.
- Consumer-owned profile preservation, unlink safety, same-subject action intents, detached-profile deletion protection and app-local onboarding races, verified in each consumer's repository.
- Full controlled client-secret cutover, destructive deployment restore and manual identity-recovery rehearsals, with production backup/rollback sign-off.

Durable deletion remains off by default. Existing legacy clients stay explicitly outside full v1 access compliance until reviewed cutover. Do not remove compatibility fields or advertise production v1 compliance while these gates remain unverified.
