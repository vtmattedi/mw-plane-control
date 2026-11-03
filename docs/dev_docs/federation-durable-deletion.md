# Durable whole-identity deletion

## State and compatibility contract

The candidate list is a transactionally captured union of historical consent and assignment applications. It is not an account registry. Internal infrastructure has no local profile and is `exempt`; an ordinary app without a callback is `unresolved` and blocks final deletion.

A deletion operation is `pending`, `legacy_review_required`, or `completed`. Each application delivery is `pending`, `delivering` under a renewable-by-reclaim lease, `confirmed_cleanup`, `no_current_mapping`, `exempt`, `failed` after the retry horizon, or `unresolved`. Only confirmed cleanup, no-current-mapping and explicit infrastructure exemption satisfy an obligation. A transport failure or queued response never confirms erasure.

One transaction disables the identity, records the original request time, revokes central sessions/codes/tokens, creates/reuses the operation and snapshots deliveries. It performs no outbound HTTP. Every delivery has one stable random event ID; retries refresh the short-lived JWT signature under the active signing key without changing operation/event identity or original request time. Outbox/status records have no cascading foreign key to the user. The same disabled subject cannot authenticate or complete new enrollment. Receivers remain responsible for current mapping, idempotent cleanup and fencing app-local handoffs.

The callback payload retains `identity.user.deleted` and signed `reason=account_deleted`, adding `operation_id`. JWT binds issuer, subject, exact client audience, jti/header/body event ID, event, reason, application ID, operation ID and original request time. Existing compliant receivers can ignore the additive operation field. `200` or `204` acknowledges required completed cleanup under the existing contract; `202` and other queued/nondefinitive responses remain retryable. A `200` JSON response with exact `outcome=no_current_mapping` is recorded separately; this acknowledges absence of a current mapping, not historical erasure. Consumers must validate all existing signed destructive bindings and ignore legacy disconnect events non-destructively.

Work claims use random lease tokens and atomic SQLite updates. Expired leases can be reclaimed after a crash; stale workers cannot record a result after their lease is replaced. Delivery uses bounded concurrency, a per-request timeout, bounded response body, no redirects and exponential backoff with jitter, capped at one hour, over a seven-day retry horizon. Exhausted work remains `failed` for operator recovery; it is never silently completed. A changed callback/client registration is `unresolved` until an operator reviews and refreshes the snapshot. Disabled/archived apps retain deletion obligations.

Production callback dialing resolves and checks addresses at connection time, rejecting private, loopback, link-local, unspecified and non-global destinations, and disables environment proxies. This protects against DNS rebinding; HTTPS host validation still uses the registered hostname. Local callback exceptions are available only outside production. Callback response bodies, tokens and raw URLs are not logged.

## Progress and recovery

Initiation returns promptly with a status capability stored in an HttpOnly, SameSite=Strict cookie scoped to `/account/deletion`. Only its hash and 24-hour expiry are stored; neither capability nor identity is put into a URL. The browser can revisit `/account/deletion` after normal sign-in sessions are revoked. Status responses are no-store and no-referrer. Operation IDs alone never authorize public access. Closing/reopening the browser retains the status cookie until expiry. Operators use separately authorized status/retry routes after that expiry or browser loss.

Legacy already-disabled identities are imported as `legacy_review_required`, with no fabricated old event IDs and no automated delivery. Operators must reconcile receiver state and original timestamps before explicitly adopting work into the new protocol. This release does not bulk-replay legacy callbacks or WOI `awaiting_contract` receipts.

`IDENTITY_DURABLE_DELETION_ENABLED=false` stages schema and code with the worker off. Enable only after disposable receiver compatibility tests, a verified production backup and documented live-client audit. With the flag on, old request-bound deletion is replaced by the durable path. Existing pending operations are durable even when the worker is disabled; disabling the worker never converts them into legacy request-bound work.

## Retention, backup and rollback

Pending, failed and unresolved work is retained until resolved. Completed records keep minimal subject/event/outcome metadata for 30 days to support status and replay correlation; capabilities expire after 24 hours. Operator retry and snapshot changes are audited. Maintenance must never purge outstanding work. Receivers retain their own idempotency/deletion barriers under their policies; no central tombstone substitutes for those barriers.

Online SQLite backup includes the outbox. Restore only in isolation first; replay receiver completion receipts newer than the backup before exposing traffic, keeping event IDs unchanged. A retry after lost acknowledgement is expected and must be idempotent. Retain original encryption keyrings and signing-key overlap.

Rollback disables worker dispatch first, preserves schema/outbox and leaves deleting identities disabled. Do not run a legacy binary that can reopen or synchronously finalize an outstanding durable deletion. Restore/reconciliation must preserve these records and never claim erasure from a skipped callback. Browser/hardware and production recovery remain separate release gates.

## Operator routes

Use existing Identity administration authorization (`identity.admin`, users read/write permissions) through `/api/identity/` on the BFF or the private Identity `/admin/api/` routes. Mutations require fresh WebAuthn and the BFF requires its existing browser CSRF proof.

- `GET /admin/api/deletions` lists up to 500 recent operations, including migrated legacy requests; only this operator view exposes the subject for reconciliation.
- `GET /admin/api/deletions/{id}` shows per-app outcomes.
- `POST /admin/api/deletion-deliveries/{event}/retry` with `{"refresh_snapshot":false}` reopens failed work; set true only after reviewing a changed registered callback. It resets the retry window without changing the event ID or original requested time.
- `POST /admin/api/deletions/{id}/reconcile-legacy` accepts `outcomes` keyed by client ID and an `evidence_reference`. Every ordinary historical candidate needs verified `confirmed_cleanup` or `no_current_mapping`; infrastructure is exempt. This records terminal operator receipts and finalizes only after all required outcomes, without emitting invented historical callbacks. An unavailable app cannot be marked complete without evidence.
- Private `GET /metrics/deletions` exposes aggregate queue age/outstanding/failure counts; the gateway blocks the metrics prefix. No subject, client or token labels are included.

Reconciliation is an operator attestation, not automated proof of application erasure. Retain the referenced support evidence in approved storage. No bulk replay is provided.
