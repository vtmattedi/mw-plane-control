# Local Identity / WOI deletion compatibility

This follow-up adds verification only. It uses the provider implementation after `81534e8` and WOI's receiver after `2ff2c6d`. No deployment configuration, migration, production route or frozen-contract byte changes. Normal durable dispatch remains disabled.

## What runs

WOI's `backend/test/identityProviderDeletion.test.ts` builds the Go fixture at `mw-identity/internal/auth/testdata/receiverfixture/main.go` into a verified OS temporary directory. The fixture initializes a fresh SQLite database, embedded migrations, random signing encryption keyring, real signing manager and real authentication/deletion service. It enables durable dispatch only within that disposable service. Discovery uses the provider's discovery document; JWKS comes from its signing manager. Only discovery/JWKS listen on a dynamic loopback port. Test commands use stdin/stdout JSON; callback registration accepts loopback HTTP only. It never loads deployment configuration or accepts an existing database path.

The consumer uses the actual authenticated WOI lifecycle route, PostgreSQL transactions, deletion receipt, account erasure and filesystem cleanup. The test requires an explicitly selected disposable PostgreSQL port and sets test-only credentials/database. It changes working directory before importing application modules so no application `.env` is loaded. Like the existing identity suite, it resets `woi_identity_test`; run these suites sequentially.

From the WOI checkout, with Go available:

```powershell
$env:WOI_TEST_PG_PORT = '<disposable PostgreSQL mapped port>'
$env:MW_IDENTITY_PROVIDER_DIR = 'D:/MattediWorks/mw-ecosystem/mw-plane-control/mw-identity'
npm --prefix backend run test:identity-provider
```

The PostgreSQL test user/database/password requirements are identical to WOI's `test:identity` suite. Use a fresh isolated container; never supply an application database endpoint. Fixture binaries, SQLite data/keyrings and upload files are temporary. Assertions retain only safe event/operation/key metadata and a one-way signature digest, never a raw bearer token. Cleanup closes servers/processes/databases before removing verified temporary directories.

## Observed results on 2026-11-04

All four cases pass:

1. A receiver filesystem error leaves both the provider operation and receiver cleanup receipt pending. The fixture closes/reopens SQLite and reconstructs the provider service/signing manager, then rotates the signing key. Retry uses the same operation/event/original request time, WOI refreshes JWKS, filesystem cleanup completes, and the provider records `confirmed_cleanup` before removing its central user.
2. A detached WOI profile and its files survive a provider event. The real receiver returns `no_current_mapping`, which the provider records separately from erasure.
3. A receiver-side HTTP 200 `pending` envelope leaves the provider user disabled and operation pending even though local cleanup finished. A normal retry uses the existing completed receipt and permits finalization.
4. A malformed HTTP 200 envelope has the same fail-closed behavior; a subsequent valid terminal response completes the original operation.

The fixture compiles and passes Go vet. The new TypeScript integration file passes an explicit no-emit typecheck, including its imported backend modules. The PostgreSQL client emits the existing deprecation warning. No new runtime defect was found by these cases.

This exercises the actual worker delivery operation via deterministic calls, advancing only its test retry deadline. The restart is a SQLite/service restart in the fixture process, not an OS crash or deployment restore. It does not certify timer scheduling, hardware/browser authentication, live registration, public HTTPS/production DNS restrictions, other receiver populations or production recovery. Existing unit/integration evidence covers leases and scheduling separately; remaining release gates stay open.

## Focused ownership and code review

The only source additions in this continuation are the fixture and consumer integration test, plus a package script. Application source is unchanged. Existing source outside these additions is traced for compatibility, not independently re-audited. Generated dependencies, compiled output and unrelated WOI frontend work are excluded.

| File | Q1: placement | Q2: separation | Q3: maintainability | Q4: comments |
| --- | --- | --- | --- | --- |
| `mw-identity/internal/auth/testdata/receiverfixture/main.go` | Keep beside the workflow's test data; absent from production entry points | One narrow fixture owns temporary DB/service control and public key discovery | Structured commands, loopback-only callback, fresh DB/keyring, bounded callback I/O and synchronized JWKS/restart access | Explain isolation, explicit test opt-in and deterministic retry deadline |
| `WOI/backend/test/identityProviderDeletion.test.ts` | Keep with actual receiver integration tests | Setup/control helpers separate from four observable scenarios | Explicit DB/provider inputs, real assertions, bounded process protocol, receipt/file/state checks and verified cleanup | Explain destructive-test isolation; avoid narrating assertions |
| `WOI/backend/package.json` | Keep test runner script with existing scripts | No dependency/runtime changes | Explicit separate runner prevents accidental concurrent resets | No comments needed |

The fixture reuses production signing, storage and delivery code instead of copying a protocol implementation. A new application service or test HTTP control API would add unnecessary ownership and exposure. The existing Go/TypeScript boundaries remain appropriate. Review found no further defect in this continuation's diff; production activation remains gated.
