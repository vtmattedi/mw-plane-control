# Federation v1.0 clean contract migration

Decision: 2026-11-04, explicitly replacing the earlier staged consumer-compatibility policy. Breaking protocol changes are authorized; destruction of identities or important persisted data is not. The frozen [Federation Contract v1.0](../MW-Identity-Federation-Contract-v1.0.md) remains byte-for-byte normative.

## Provider behavior

- Every ordinary application uses its stored Public (`default_allow`) or Private (`default_deny`) access policy. Explicit denial wins. Global superadmin status, retired signup metadata and the old per-client migration flag grant no ordinary-app exception. MW administration privileges require explicitly classified infrastructure; reserved audiences alone cannot grant the infrastructure exemption.
- Federation assurance comes only from requested ACR, freshness and verified proof. There is no application-wide Strong gate. Existing explicit BFF/admin fresh-WebAuthn gates remain. Registration uses only the global signup switch.
- Registry APIs/models/UI no longer expose or accept `authentication_policy`, `allow_public_signup` or `access_policy_v1`. Old CLI policy/signup flags and the `federation-cutover` executable/store operations are removed. Current scripts request Strong through `acr_values`.
- OIDC authorization requires a nonblank nonce of at most 1024 bytes. Its opaque value is preserved exactly. Issuance and exchange reject nonce-less artifacts; ID tokens always carry the bound nonce.
- Every whole-account deletion enters the durable workflow: atomically freeze the identity, revoke central credentials, snapshot candidate applications, persist stable events and issue scoped progress access. There is no request-bound deletion implementation or synchronous fallback.
- `IDENTITY_DELETION_DISPATCH_ENABLED` is a startup dispatch switch, default true. Production Compose requires an explicit true or false value. Pausing dispatch does not change how requests are accepted or persisted. No-obligation operations can finalize; operations requiring receiver outcomes remain pending until acknowledged. Resume dispatch by starting with it enabled after deployment checks.
- Every receiver acknowledges with HTTP 200 and exactly `{"version":1,"outcome":"confirmed_cleanup"}` or `{"version":1,"outcome":"no_current_mapping"}` after its required work completes. Empty 200, 204, unversioned bodies and pending/malformed/unknown responses never authorize finalization. There is one parser and one delivery state machine.
- Consent revocation is non-destructive. `application_disconnected` must never authorize deletion. This security invariant is retained regardless of compatibility policy.

WOI, Stackport and other consumers must validate the new contract before integration: signature/JWKS, exact issuer/audience/subject, nonce/state/PKCE, achieved ACR and freshness, action/session binding and signed destructive event bindings. Consumer readiness is a deployment prerequisite. Preserving old WOI responses or old consumer behavior is not a provider merge requirement. This provider change does not publish, deploy or automatically connect consumers.

## Persisted data and migration 020

The migration preserves identities/passwords, application IDs and secrets, stored access values, assignments/roles/denials, consent history, signing/TOTP material, existing deletion operations/deliveries, progress hashes/expiry, original request times and event IDs. Historical registry columns remain physically present as inert evidence; runtime policy does not read/write or branch on them. No table/column containing domain or recovery data is dropped.

Migration `020_federation_v1_contract.sql` invalidates pre-migration authorization codes and opaque access tokens, requiring fresh consumer transactions. These transient grants are deliberately revoked; they are not profile or consent data. It records one transactional `identity.federation.contract_migrated` audit with grant counts and recovery-import count. Reapplying migrations does not repeat the cutover or audit.

A prior request-bound deployment could have frozen an identity after migration 018 without leaving an operation. Migration 020 imports such orphaned requests as `legacy_review_required`, preserving the exact original timestamp. It creates no delivery/event IDs and dispatches nothing for them. Existing durable operations remain untouched. The retained reconciliation API is an actual persisted-data recovery tool; operators supply evidence-backed outcomes without fabricating historical callbacks. It is not a legacy consumer protocol.

Back up and rehearse the migration on an isolated database before deployment. Do not run old/new writers against the same store. Protocol rollback to binaries that reintroduce ordinary-app bypasses, destructive disconnect or request-bound deletion is unsafe. Preserve the outbox and disabled subjects during recovery; no automatic operation replays or profile deletions are added by this migration.

## Design/change decisions

The existing Go package ownership remains appropriate: `oidc` evaluates requests/evidence, `applications` and `identityadmin` own registry inputs, `auth` owns deletion orchestration/protocol handling, and `store/sqlite` owns atomic state. The React console consumes the registry contract. No new generic service/repository layer is needed.

| Finding | Decision and benefit | Effort/risk/reversibility | Verification |
| --- | --- | --- | --- |
| Go/SQL could select old eligibility by client flag | RESTRUCTURE: one policy for all ordinary clients; narrow explicit infrastructure classification | Small; high authorization consequence. Revert source only with a deliberate security/contract decision | Go/SQL private, deny, superadmin and misclassified-admin-audience regressions |
| App Strong and signup flags still affected runtime/API | RESTRUCTURE: remove fields/gates/UI/CLI; per-request assurance and global signup are authoritative | Medium; intentional breaking API. Physical stored values preserved | API rejects retired controls, omits them from responses; signed password/MFA/passkey round trips with retired DB fields set |
| Deletion had two implementations and two response policies | RESTRUCTURE: delete request-bound code/tests; keep durable state/delivery/parser owners | Medium; high destructive-workflow consequence. No data-table deletion; old-path rollback unsafe | Dispatch pause still queues/freezes; resume uses stable operation; missing callbacks never finalize; strict receiver matrix |
| Nonce could be omitted | RESTRUCTURE: require it before login and through issuance/exchange; echo opaque nonce exactly | Small; intentional rejection of incomplete consumers | HTTP missing/blank/oversized rejection; direct issuance/exchange guard; real signed token nonce equality |
| Earlier failed requests could lack durable recovery rows | RESTRUCTURE: import only missing operations for operator review | Medium; migration transaction/restore consequence. Original data retained | Actual 019→020 upgrade with rollback fault, idempotence, exact snapshots, orphan import and no fabricated events |
| Existing outbox, proof and admin security boundaries | KEEP: retain atomic freeze/audit, capabilities, leases, receipt fencing and fresh-WebAuthn mutations | Avoids unnecessary ownership churn | Full existing suites plus cross-service worker/WOI tests |
| Physical removal of historical columns and old records | DEFER: no implementation need; preserve important evidence and recovery | Requires a separate reviewed data-retention decision | Migration snapshots verify retained bytes |

## Verification and scope

Identity and BFF tests/vet pass. The Linux production image builds, excludes the removed cutover executable, and rejects the retired CLI flag; changed PowerShell scripts parse successfully. Frontend typecheck, lint, all 144 tests and production build pass, retaining the existing unrelated hook/bundle warnings. The real Identity→WOI compatibility suite passes four cases against isolated PostgreSQL/SQLite, actual signatures, receipt/file cleanup, service restart/key rotation and invalid acknowledgements. Passing this optional consumer check does not make other consumers ready or gate provider merge on old behavior. Migration recovery, authorization, nonce and strict callback regressions cover the revised decision.

The frozen contract and migrations 001–019 are unchanged. No production configuration/database is read or deployed. Hardware/browser ceremonies, public production transport, live consumer integration and controlled production recovery remain deployment checks.

The source review covers this change against `bb29ec8`, including removed files, tests, startup/build/script configuration and admin forms. Prior historical plans/reports are labeled or superseded by this decision. Unchanged modules and consumer source are traced as dependencies rather than independently re-audited. Generated dependencies, build output and unrelated work are excluded. The per-file review inventory below records placement, separation, maintainability and comment decisions for the changed source/configuration scope.


## Per-file change review

Reviewed 53 changed source/test/build/configuration paths, including removed files; none are unreviewed within this diff scope. Documentation explains the revised contract and labels historical evidence; generated output is excluded. Q1 placement, Q2 separation, Q3 maintainability, Q4 necessary comments:

| File | Q1 | Q2 | Q3 | Q4 |
| --- | --- | --- | --- | --- |
| `.env.example` | KEEP: operator configuration | Dispatch meaning documented | False pauses outbound work only | Explain durable acceptance while paused |
| `mw-identity/Dockerfile` | KEEP: image build ownership | Removed obsolete executable stages | No stale cutover binary shipped | No extra comment needed |
| `mw-identity/cmd/create-application/main.go` | KEEP: trusted provisioning CLI | Removed policy/signup flags | One registry input contract | Existing provisioning comments retained |
| `mw-identity/cmd/federation-cutover/main.go` | REMOVE: obsolete consumer cutover CLI | Migration 020 owns real data cutover | No per-client compatibility operation | Historical docs superseded |
| `mw-identity/cmd/server/main.go` | KEEP: service composition | Wire dispatch setting only | No alternate deletion selection | Existing lifecycle ordering retained |
| `mw-identity/internal/applications/model.go` | KEEP: registry contract owner | Removed retired fields | Lifecycle requirement independent of signup | Updated registration/lifecycle rationale |
| `mw-identity/internal/applications/service.go` | KEEP: registry validation | No app-wide assurance normalization | Only access/lifecycle metadata accepted | Existing URI/secret invariants retained |
| `mw-identity/internal/applications/service_test.go` | KEEP: registry behavior tests | Drop removed signup-policy case | Callback/secret validation remains | No extra comment needed |
| `mw-identity/internal/auth/account_lifecycle.go` | KEEP: account/consent operations | Single durable dispatch entry | No request-bound branch | Non-destructive consent rationale retained |
| `mw-identity/internal/auth/account_lifecycle_test.go` | REMOVE: old mixed callback tests | Durable delivery and username tests relocated | No obsolete three-attempt expectations | Replacement tests state current behavior |
| `mw-identity/internal/auth/deletion_delivery.go` | KEEP: signed callback adapter | Removed synchronous signer/retry duplicate | One bounded sender and signed durable payload | Keep original-event binding rationale |
| `mw-identity/internal/auth/deletion_delivery_test.go` | KEEP: callback adapter tests | One attempt; scheduler retries elsewhere | Terminal/retryable and signed bindings tested | No implementation narration |
| `mw-identity/internal/auth/deletion_legacy.go` | REMOVE: duplicate destructive workflow | Durable operation owns state | No alternate acknowledgement/finalization path | Limits move to current runbook |
| `mw-identity/internal/auth/deletion_operation.go` | KEEP: durable initiation/operator audit | Setting controls dispatch only | Capabilities/audits remain unchanged | No compatibility comment retained |
| `mw-identity/internal/auth/deletion_response.go` | KEEP: receiver protocol owner | One strict parser | Versioned terminal 200 only | Comment states terminal invariant |
| `mw-identity/internal/auth/deletion_worker.go` | KEEP: scheduling/lease owner | Startup pause only | Retry/fencing semantics unchanged | Keep shutdown persistence rationale |
| `mw-identity/internal/auth/registration_test.go` | KEEP: username validation tests | Moved out of deletion test file | Existing username edge cases retained | No extra comment needed |
| `mw-identity/internal/auth/service.go` | KEEP: auth/session owner | Dispatch field renamed; no auth rewrite | State setting is not a workflow mode | Existing proof/login notes retained |
| `mw-identity/internal/auth/store.go` | KEEP: operation/store contracts | Signup context has no per-client flag | No persistence policy leakage | Existing data semantics retained |
| `mw-identity/internal/auth/testdata/receiverfixture/main.go` | KEEP: isolated receiver fixture | Uses current registry/dispatch API | Actual worker tests still pass | Explicit disposable fixture setting retained |
| `mw-identity/internal/config/config.go` | KEEP: configuration parsing | Operational setting renamed internally | Environment default is enabled; production Compose requires an explicit value | Environment example explains meaning |
| `mw-identity/internal/httpapi/account_deletion_test.go` | KEEP: self-deletion journeys | Paused/enabled cases use one workflow | Password signup and reauth tests retained | No legacy-mode label |
| `mw-identity/internal/httpapi/consent_test.go` | KEEP: real consent/OIDC journey | Public access represented explicitly | No signup flag access shortcut | Existing consent/replay notes retained |
| `mw-identity/internal/httpapi/deletion_test.go` | KEEP: scoped progress tests | Use dispatch setting name | Session revocation/public privacy retained | No extra comment needed |
| `mw-identity/internal/httpapi/freshness_test.go` | KEEP: precise freshness regression | Fixture uses current registry | Same-second old session rejected | Existing temporal invariant retained |
| `mw-identity/internal/httpapi/identity_admin.go` | KEEP: admin HTTP boundary | Removed deprecated DTO controls | Strict decoder rejects old fields | Existing permission notes retained |
| `mw-identity/internal/httpapi/identity_assignments_test.go` | KEEP: authenticated admin API tests | Shared fixture for assignment/registry behavior | Deny protections and retired input rejection tested | Explain rejection has no registry mutation |
| `mw-identity/internal/httpapi/nonce_test.go` | KEEP: authorization boundary regression | Focused missing/blank/oversized cases | Reject before login with bound state | Comment explains absent auth handler |
| `mw-identity/internal/httpapi/registration_test.go` | KEEP: global signup journeys | Removed retired fixture properties | Global switch and private navigation retained | Existing signup context notes retained |
| `mw-identity/internal/identityadmin/model.go` | KEEP: admin domain DTOs | Removed retired controls | One writable registry contract | No redundant field comments |
| `mw-identity/internal/identityadmin/service.go` | KEEP: registry/permission operations | Remove compatibility preservation branches | Lifecycle and infrastructure protections retained | Updated policy behavior in current guide |
| `mw-identity/internal/oidc/admin_access.go` | KEEP: Go eligibility owner | Removed client mode/signup bypass | Infrastructure exception requires explicit class | No legacy exception rationale retained |
| `mw-identity/internal/oidc/service.go` | KEEP: OIDC protocol/evidence owner | No app-wide Strong path | Required nonce; ACR/proof/freshness authoritative | No obsolete gate comments |
| `mw-identity/internal/store/sqlite/account_lifecycle_test.go` | KEEP: consent/deletion fixture | Uses current registry model | Per-app token/consent isolation retained | Existing fixture scope retained |
| `mw-identity/internal/store/sqlite/applications.go` | KEEP: registry persistence | Retired columns are not read/written | IDs/secrets/navigation and audit retained | Current metadata warnings retained |
| `mw-identity/internal/store/sqlite/applications_test.go` | KEEP: persistence/secret tests | Removed retired API assertions | Roundtrip and restrictive policy behavior retained | No extra comment needed |
| `mw-identity/internal/store/sqlite/assurance_test.go` | KEEP: real signed proof pipeline | Add nonce failure/old metadata regressions | Stored Strong ignored; exact nonce and proof tested | Explain direct artifact exchange rejection |
| `mw-identity/internal/store/sqlite/consent.go` | KEEP: atomic credential predicates | Single Go/SQL-equivalent access model | No ordinary superadmin/signup/client-mode bypass | Keep shared-predicate rationale |
| `mw-identity/internal/store/sqlite/consent_test.go` | KEEP: historical migration/consent tests | Fix pre-migration boundary; strict private case | Assignments/history survive; old grants revoked | Updated current-cutover expectations |
| `mw-identity/internal/store/sqlite/contract_migration_test.go` | KEEP: actual database upgrade test | Snapshots, fault injection and recovery import together | Opaque secrets/outbox preserved; audit idempotent | Explain atomic rollback and orphan recovery |
| `mw-identity/internal/store/sqlite/deletion_test.go` | KEEP: workflow/recovery integration | Remove request-bound tests; enforce one protocol | Pause queues; strict responses; recovery/leases retained | Keep restart/rollback/capability notes |
| `mw-identity/internal/store/sqlite/federation.go` | REMOVE: compatibility cutover storage | Numbered migration owns upgrade | No retired policy mutation APIs | Historical methods documented as removed |
| `mw-identity/internal/store/sqlite/federation_test.go` | KEEP: Go/SQL access fencing | Direct policy setup tests all registrations | Superadmin/denial/infra classification checked | No per-client cutover expectation |
| `mw-identity/internal/store/sqlite/identity_admin.go` | KEEP: transactional registry updates | Removed writes and superadmin revocation exemption | Restrictive policy fences every ordinary user | Existing atomic update notes retained |
| `mw-identity/internal/store/sqlite/registration.go` | KEEP: signup context persistence | Read no retired signup flag | Global registration creates no assignment | Existing account transaction rationale retained |
| `mw-identity/migrations/020_federation_v1_contract.sql` | KEEP: numbered data migration | Credential cutover and safe recovery import atomic | Preserve data; no invented event IDs; audit once | Explain transient grants and orphan imports |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.test.tsx` | KEEP: actual console interaction tests | Fixture drops obsolete response fields | Ordinary denial and MW admin protection retained | Existing interaction-test scope retained |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx` | KEEP: registry detail UI | Remove retired form fields and migration-mode banner | Public/Private applies to every ordinary app | Current incomplete-registration copy |
| `mw-web-admin/src/pages/Control/ResourcePage.tsx` | KEEP: ordinary registry creation UI | Remove dead signup/auth-policy state | Required callback always shown for ordinary apps | No extra comment needed |
| `mw-web-admin/src/pages/Control/pages.tsx` | KEEP: resource list configuration | No application assurance column | Access label matches server contract | No extra comment needed |
| `scripts/e2e-step-08.ps1` | KEEP: module integration rehearsal | Remove obsolete request property | Payload follows strict registry API | Existing scope notes retained |
| `scripts/e2e-step-12.ps1` | KEEP: auth/recovery rehearsal | Request ACR instead of client policy | Accept credential setup or challenge; nonce explicit | Replace compatibility-path comments |
| `scripts/sanity-step-06.ps1` | KEEP: administration rehearsal | Remove obsolete request properties | Payloads follow strict API | Existing checks retained |


Final diff review found no further defect in the revised implementation scope. The upgrade review finding for orphaned deletion requests is covered by migration 020 and its fault-injected preservation test.


## Dispatch operations follow-up (2026-11-04)

The provider now reads only `IDENTITY_DELETION_DISPATCH_ENABLED`, default true. The retired environment name has no alias. Both production Compose manifests require an explicit value; development defaults to true. The [deletion runbook](federation-durable-deletion.md) documents emergency pause, restart/recreation and private metrics. Consumers must comply before connection; no per-application transition controls were introduced.

Review scope: the complete follow-up diff against `e51080f`, including configuration, tests and operator documentation. All 13 changed source/test/configuration files below were reviewed; no in-scope files remain unreviewed. Unchanged modules were traced only as dependencies; generated output and historical review records are excluded. No blocking findings. KEEP the existing ownership: SQLite computes persisted counts, the auth service combines those counts with its startup setting, and HTTP renders the aggregate. No new abstraction or polling worker is needed. Durability, protocol outcomes, migration transactions, audit, progress credentials and worker leases are unchanged.

| File | Q1 placement | Q2 separation | Q3 maintainability | Q4 comments |
| --- | --- | --- | --- | --- |
| `.env.example` | KEEP operator configuration | One global setting | Explicit true and emergency pause | Explains environment recreation |
| `docker-compose.yml` | KEEP production composition | No workflow branching | Reject missing explicit setting | Environment/runbook explain operation |
| `docker-compose.stackport.yml` | KEEP managed production composition | No app-level controls | Same required setting; no ports added | Shared runbook is sufficient |
| `docker-compose-dev.yml` | KEEP development composition | Same provider setting | True fallback matches code | No redundant inline note |
| `mw-identity/internal/config/config.go` | KEEP environment parser | One boolean, no old alias | Existing invalid-value error retained | Runbook documents default |
| `mw-identity/internal/config/config_test.go` | KEEP parsing regressions | Focused default/override/error cases | True/false/zero/invalid covered | No narration needed |
| `mw-identity/internal/auth/service.go` | KEEP service construction | No worker started by constructor | Constructor default matches config | No extra note needed |
| `mw-identity/internal/auth/deletion_operation.go` | KEEP operation/dispatch owner | Compose metrics here, away from HTTP/SQL | Error propagates; no background side effects | States setter is startup-only |
| `mw-identity/internal/auth/testdata/receiverfixture/main.go` | KEEP disposable fixture | Explicit test configuration | Independent of deployment environment | Removed obsolete opt-in claim |
| `mw-identity/internal/httpapi/deletion.go` | KEEP HTTP boundary | Delegates aggregate composition | Storage errors remain HTTP 503 | No extra note needed |
| `mw-identity/internal/httpapi/deletion_metrics_test.go` | KEEP endpoint regression | Real service/SQLite behind HTTP | Pause before/after request, resume, receipt and finalization; privacy checked | Explains receipt/finalization distinction |
| `mw-identity/internal/store/sqlite/deletion.go` | KEEP persisted queue statistics | Count operations independently of deliveries | Single SQL snapshot includes zero-delivery recovery work | Test/runbook explain independent count |
| `mw-identity/internal/store/sqlite/deletion_test.go` | KEEP actual recovery migration test | Assert count at recovery boundary | Review-only operations remain visible without fabricated deliveries | Existing recovery rationale sufficient |

Verification: full Identity `go test ./...` and `go vet ./...` pass, including default/override parsing and private metric transitions. Focused recovery metrics regression passes. Compose rendering passes with true and false for all manifests, rejects missing production values, and defaults development to true, using only the isolated review environment. The frozen contract hash is unchanged; there are no migration changes. No production deployment was performed. Operators must scrape/alert on the private metric; no live administrative pause endpoint was added.
