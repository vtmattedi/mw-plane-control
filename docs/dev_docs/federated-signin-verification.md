# Federated sign-in verification — 2026-10-27

Implemented locally across Identity/control plane, WOI and the bilingual
showcase. At the validation checkpoint, no commit, push, deployment, production
database/policy change or real account deletion had been performed. Subsequent
commit/sync and integration into main were authorized separately by the user. WOI's pre-existing ENVIRONMENT.md changes were
preserved byte-for-byte; that file was not edited for this work.

## Delivered behavior

- Identity separates consent from administrator assignments. External users
  consent explicitly; public enrollment admits new and existing MW identities.
  Denials/private-app/admin-audience boundaries remain enforced.
- Disconnect revokes this client's consent/codes/tokens, sends no callback,
  preserves administrator assignments and retains whole-account target history.
  Consent versions fence code/token/delegation writes and reads, including a
  code exchange already in flight. Trusted internal credentials are migrated
  onto tracked consent rather than bypassing revocation.
- WOI disconnect/reconnect preserves the same profile/data/role/session. WOI's
  own deletion requires recent same-subject MW authentication and a distinct
  one-use final confirmation. Cleanup/cutoffs survive restart; fresh onboarding
  creates a new baseline profile without old ownership or privileges.
- Only a signed account_deleted reason authorizes destructive lifecycle
  processing. Legacy disconnect assertions are ignored. Unsigned or altered
  destructive reasons fail closed; callback rollout must be coordinated.
- Portal, admin field descriptions, WOI integration docs and both showcase
  languages distinguish disconnect, app deletion and whole-MW deletion. The
  showcase uses an explicitly labelled illustration instead of its obsolete
  destructive-removal screenshot.

## Automated checks

All commands below passed on this workspace:

| Directory | Command | Result |
| --- | --- | --- |
| mw-plane-control/mw-identity | go test ./... | PASS |
| mw-plane-control/mw-identity | go vet ./... | PASS |
| mw-plane-control/mw-bff | go test ./... | PASS |
| mw-plane-control/mw-web-admin | npm run typecheck; npm run test:run; npm run build | PASS; 25 test files / 132 tests |
| working-on-it/backend | npm run build | PASS |
| working-on-it/backend | WOI_TEST_PG_PORT=<disposable mapped port> npm run test:identity | PASS; 14 tests |
| working-on-it/frontend | npm run build | PASS; TypeScript, browser and SSR builds |
| mw-showcase | npm run typecheck; npm run build | PASS; both localized static routes |

New coverage includes real provider consent/token exchange/re-consent,
unauthenticated max_age=0 entry, CSRF, stale consent/code/token fencing,
assignment preservation, public default_deny eligibility, historical migration,
internal legacy-token continuity, destructive signed-reason binding, final
deletion intent/cancel/logout, stale handoffs, cleanup pending and fresh signup.

One test initially regenerated a local JWT between intent creation and deletion
and failed the session-fingerprint check when iat crossed a second boundary.
The fixture now preserves the same session token, matching the browser flow.

Existing nonblocking output: pg warns about overlapping client.query calls;
Vite reports the existing large WOI browser bundle. Generated WOI version/build
metadata changes from prebuild were restored after verification.

## Actual-provider browser acceptance

Used the real local Identity server and policy implementation on its established
7100 port, WOI's real backend on 4500 and SSR/frontend on its established router
port 5995. Provider SQLite lived under the existing workspace .local directory.
PostgreSQL was a dedicated postgres:16-alpine disposable container, bound only
to loopback with a Docker-assigned host port and the documented test database
and credentials. Outbound email was disabled. The existing unrelated process
on port 5173 was left alone. Initial raw-Vite captures were replaced by the
proper SSR flow; no mock issuer was used for the acceptance below.

Verified in Chromium:

1. MW registration, explicit consent and WOI username onboarding.
2. Create a representative project; disconnect with WOI open. The local ID,
   existing session and project remain usable.
3. Re-consent and reopen the SAME local ID with its data.
4. Delete inside WOI after a real max_age=0 authentication round trip and final
   confirmation. Old access ends, and subsequent explicit registration produces
   a DIFFERENT local ID with role=user and no deleted project.
5. Delete the disposable whole MW identity separately. Identity reports completed
   deletion after the actual signed callback is accepted by WOI; the old WOI
   session is rejected.
6. In a subsequent isolated run, lock a real owned upload file on Windows.
   App deletion returns pending; the browser displays that state and a new
   sign-in is blocked. Stop WOI, release the lock and restart it: startup cleanup
   completes the persisted plan and fresh onboarding succeeds. PostgreSQL pending
   cleanup count changes from 1 before restart to 0 afterward.
7. Render English/Portuguese showcase People sections at desktop/mobile widths
   from the built static artifact; inspect the revised copy and illustration.

The browser run found two issues that handler-only checks did not expose:
consent POST navigation needed a CSP-safe HTML handoff, and max_age=0 entry
without a session cookie needed its request marker before the first login.
Both are fixed; the latter has a permanent HTTP regression test.

## Evidence and provenance

Local-only evidence is under D:/MattediWorks/.local/federation-evidence. These
are screenshots of local test infrastructure, not production acceptance:

- identity-connected-en.png; identity-connected-mobile-en.png;
  identity-connected-mobile-pt.png; identity-disconnected-en.png;
  identity-reconsent-en.png.
- woi-delete-intent.png; woi-delete-final-desktop.png;
  woi-delete-final-mobile.png; woi-deleted-mobile.png;
  woi-fresh-profile-mobile.png; whole-mw-account-deleted.png.
- woi-cleanup-pending-mobile.png; woi-pending-signin-blocked.png;
  woi-cleanup-recovered-registration.png; woi-recovered-profile.png.
- showcase-en-desktop.png; showcase-en-mobile.png;
  showcase-pt-desktop.png; showcase-pt-mobile.png.

acceptance-result.json records one completed happy-path run and the separate
whole-account confirmation. Test databases were reset between regression and
later recovery runs; numerical IDs across separate runs are not comparable.
Temporary browser scripts/state and generated binaries remain outside source
repositories. Test stack processes and the disposable PostgreSQL container were
shut down after validation; screenshots and result files remain for review.

## Rollout and remaining limits

Read [federated-signin-rollout.md](federated-signin-rollout.md) before deploying.
Migration 013 adds/backfills consent history and artifact versions; WOI's additive
intent-table SQL is backend/sql/001_account_deletion_intents.sql. Consumer-first
deployment temporarily rejects old providers' unsigned destructive callbacks;
upgrade Identity and retry pending whole-account deletion rather than weakening
validation. Old destructive binaries are not a compatible routine rollback.

Live registration, internal-client classification/inventory, production callback
reachability and live BFF/mail/OAuth federation remain deployment checkpoints.
Local migration/internal-token tests and BFF suites do not certify those live
registrations. Existing whole-account operation-ID/retry limitations, lack of
instant remote logout and backup/export erasure limits remain explicitly scoped.
