# Deletion authentication, receiver and recovery review

> Historical follow-up review. The later [clean contract migration](federation-v1-clean-migration.md) removes the legacy deletion mode and acknowledgement compatibility described here.

The subsequent [real-provider/WOI compatibility verification](federation-receiver-compatibility.md) adds four cross-service cases and a separate review inventory. It does not expand this document's original source-review scope or enable production dispatch.

Scope: the provider follow-up against `d6122ab` on `feat/identity-federation-v1` and the WOI receiver follow-up against `89fdb13` on `fix/identity-deletion-receiver`. Reviewed the changed units and their callers, proof/session handling, consent history, callback outcomes, capability persistence, receiver ledger and cleanup ordering. The inventory below covers all 24 changed source/test/template files. Other modules were traced as dependencies but are not independently re-audited here. Generated files, dependencies and WOI's unrelated frontend work are excluded. Documentation changes describe behavior rather than add runtime dependencies.

## Decisions and verification

The language-native Go packages and WOI's transaction-owned workflow remain appropriate. Extract response parsing and bounded reads into `auth/deletion_response.go`; callback signing/HTTP delivery into `auth/deletion_delivery.go`; request-bound orchestration into `auth/deletion_legacy.go`; durable initiation/operator audit into `auth/deletion_operation.go`. `deletion_worker.go` now owns scheduling, claims, retry timing and result persistence. Initial/terminal state policy belongs with `deletion_model.go` and is shared across modes. Keep SQL transaction ownership in `store/sqlite`; no schema/interface framework or microservice split is justified.

Move the destructive portal POST into `httpapi/account_deletion.go` and the shared account-page/POST assurance decision into `account_authentication.go`. Login remains the existing explicit `auth=reauth` flow, with a return to the deletion section and a new final confirmation. CSRF validation precedes the freshness redirect. Password registration now labels its existing password proof consistently. Recovery/missing/mismatched/future/stale proofs cannot authorize deletion; Standard, password+TOTP and passkey proof can. Non-destructive consent revocation retains session/CSRF requirements without a recent/Strong gate.

WOI's `identityDeletion.ts` owns the locked ledger transaction, current-mapping check, account erasure and completion ordering. `mwIdentity.ts` keeps request validation and signature checks, then emits only the completed operation's terminal outcome. The existing event's `deletedUserId` distinguishes cleanup from no mapping on retries; no new lookup can target a replacement account. `finishDeletion` must persist exactly one completion receipt before a new acknowledgement. Existing sign-in barriers, fingerprints, timestamp columns, subject locks and non-destructive legacy disconnect behavior remain.

The progress credential is issued only while the operation has no credential. Repeated requests return no replacement token; the original hash/expiry remain valid through delivery retries and final user cascade. Retain the existing audited initiation transaction, random stable event IDs, atomic claims, lease fencing, finalization checks and legacy reconciliation. No migration file or frozen-contract bytes changed. Do not change user data/schema merely to make the extraction uniform.

Validation passed:

- Full Identity `go test ./...` and `go vet ./...`, including real database/HTTP regressions for password-only signup then deletion in both modes; stale password session then reauthentication and new confirmation; old-CSRF replay rejection; stale-password consent revocation; missing-callback legacy failure; status credential preservation and expiry; and the existing migration/lease/restart/backup/audit coverage.
- WOI backend typecheck and all 18 identity integration tests against a separate disposable PostgreSQL container, real RS256 assertions, HTTP, files and sockets. Tests cover failed cleanup/retry, duplicate delivery outcomes, detached/no-current-mapping preservation, replacement-profile safety, waiting for filesystem completion and a missing completion receipt. The suite reports its existing PostgreSQL client deprecation warning.
- Diff whitespace checks, source UTF-8 checks and frozen-contract digest verification. No frontend application logic changed; the portal is rendered by the Identity HTTP/template tests.

No further defect was identified in this follow-up scope. Browser/hardware authentication and live multi-consumer rollout/recovery remain unverified release gates. Durable dispatch remains disabled by default in configuration, all Compose variants and `.env.example`; no production setting was changed. Legacy fallback reports only central deletion and known candidates (`legacy_pending`/`legacy_completed`, bounded audit scope), never complete cross-application cleanup or durable delivery. Unknown/missing ordinary callbacks cannot authorize final deletion.

## Source ownership inventory

Paths below are relative to `mw-identity/` unless prefixed `WOI/`. Each row records the final placement, cohesion, maintainability and comment decision after implementation.

| File | Q1: placement | Q2: separation | Q3: maintainability | Q4: comments |
| --- | --- | --- | --- | --- |
| `internal/auth/account_lifecycle.go` | Keep account/consent entry points | Changed: dispatch extracted | Thin orchestration; explicit mode selection | Keep consent non-destruction invariant |
| `internal/auth/account_lifecycle_test.go` | Keep callback/auth regressions | Keep focused tests | Terminal expectation follows protocol | Keep attempt/signature rationale |
| `internal/auth/deletion_model.go` | Keep state policy and store contract | Shared initial/terminal decisions | Missing callback distinct from exemption | Added exemption/failure invariant |
| `internal/auth/deletion_worker.go` | Keep durable scheduling | Changed: response/initiation extracted | Claims/backoff/fencing remain explicit | Keep shutdown persistence rationale |
| `internal/auth/deletion_delivery.go` | New callback integration owner | One bounded HTTP sender for both modes | Signing formats and retry bounds preserved | No narration needed |
| `internal/auth/deletion_response.go` | New protocol response owner | Parsing/read bounds separate from scheduling | Strict envelope; no ambiguous fallback | Keep legacy acknowledgement contract |
| `internal/auth/deletion_legacy.go` | New compatibility workflow owner | Separate limited mode from durable dispatch | Explicit terminal aggregation and audit scope | Limits documented in runbook |
| `internal/auth/deletion_operation.go` | New initiation/operator audit owner | Worker no longer owns browser credential issuance | Return token only if stored | No redundant flow comments |
| `internal/auth/service.go` | Keep authentication/session ownership | No broad authentication refactor needed | Registration method matches existing proof | Switch case is self-explanatory |
| `internal/auth/store.go` | Keep account/store data contracts | Internal flag carried without SQL leakage | Infrastructure flag hidden from JSON | Field needs no extra comment |
| `internal/httpapi/identity_auth.go` | Keep login/account/consent transport | Changed: destructive handler extracted | Page receives one shared assurance decision | Retain existing login/return invariants |
| `internal/httpapi/account_authentication.go` | New account assurance policy owner | Shared by page and mutation | Explicit proof/method/time/window checks | Names express intent |
| `internal/httpapi/account_deletion.go` | New destructive portal transport owner | Separate from general login rendering | CSRF then freshness then service call | Existing ordering is clear |
| `internal/httpapi/account_deletion_test.go` | New HTTP journey regressions | Helpers limited to these journeys | Signup/reauth/new-confirmation/revocation tested | No implementation-mirroring commentary |
| `internal/store/sqlite/account_lifecycle.go` | Keep consent candidate persistence | No unnecessary repository split | Infrastructure travels with candidate snapshot | Keep historical-consent invariant |
| `internal/store/sqlite/deletion.go` | Keep transactional outbox/recovery owner | Keep coupled mutation guarantees together | Conditional one-time credential issue; exact row checks | Added no-rotation/no-extension invariant |
| `internal/store/sqlite/deletion_test.go` | Keep durable/legacy DB integration regressions | Existing fixture reused | Tests retain original capability through completion | Explain operator retry and recovery cases |
| `web/static/account.html` | Keep account view | Branch only deletion confirmation vs reauth | Explicit link; destructive form only when eligible | No template narration needed |
| `web/static/messages.go` | Keep bilingual catalog | No policy in translations | Matching keys; clear new-confirmation copy | No extra comments needed |
| `web/static/render.go` | Keep template data boundary | One boolean, no session policy in template | Existing rendering API retained | No redundant field comment |
| `WOI/backend/src/mwIdentity.ts` | Keep OIDC and lifecycle authentication transport | Changed: deletion operation extracted | Signature checks stay before operation | Keep security-boundary comments |
| `WOI/backend/src/identityDeletion.ts` | New receipt/cleanup operation owner | Transaction and post-commit completion explicit | Outcome from saved evidence, not new lookup | Added retry/replacement invariant |
| `WOI/backend/src/accountDeletion.ts` | Keep shared local erasure/file cleanup | No new copy of cleanup machinery | Completion update must affect one receipt | Keep transaction/filesystem ordering notes |
| `WOI/backend/test/mwIdentity.test.ts` | Keep real receiver integration suite | New cases use isolated existing fixture | Retry/no-mapping/order/receipt verified | Retain destructive test isolation guard |
