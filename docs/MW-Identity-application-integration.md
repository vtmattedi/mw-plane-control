# Integrating an application with MW Identity

The [frozen Federation Contract v1.0](MW-Identity-Federation-Contract-v1.0.md) is normative. This source implements staged provider changes; [conformance evidence](dev_docs/MW-Identity-Federation-v1-Compliance.md) separates automated checks from live release gates. Existing clients retain compatibility settings until explicit review and cutover. A deployed installation is not automatically v1 compliant.

## 1. Register and approve the application

An authorized administrator registers an ordinary application with a stable `client_id`, OAuth type `confidential` or `public`, Public/Private access, exact HTTPS redirect/logout URLs, allowed scopes/claims/audiences and a required `delete_callback_uri`. Optional `account_management_uri` and `account_deletion_uri` are static navigation links. Applications do not self-register.

OAuth Public means no client secret can be protected; application Public means eligible people can authorize without an explicit assignment. These dimensions are independent. A confidential secret is generated once, stored hashed by Identity and kept server-side by the app. Public OAuth clients receive no secret. Both require S256 PKCE.

Public access maps to `access_policy=default_allow`; Private maps to `default_deny`. For v1 clients, explicit deny wins, Private requires explicit allow, and ordinary-app superadmins have no exemption. Assignments grant authentication eligibility, never local roles. Public identities remain excluded from privileged MW admin audiences.

New applications use `access_policy_v1=true`. Migration 014 leaves existing clients in legacy mode until an operator reviews signup-based access and superadmin exceptions. Do not describe a legacy default-deny client as fully Private until cutover. See the [delivery record](dev_docs/MW-Identity-Federation-v1-Delivery.md).

Infrastructure clients are provisioned through the audited host-operator `create-application --internal` workflow. Normal API/forms cannot create or promote infrastructure or privileged Identity registrations. Private ordinary apps are not infrastructure. Existing trusted infrastructure remains intact; ordinary callback-less legacy apps remain unresolved and must add a callback on normal edits.

Registry URLs reject credentials/fragments and have bounded length. HTTPS is required, with an explicit HTTP localhost development exception. Account navigation embeds no subject, token or deletion authority. Production callback dialing additionally rejects nonpublic network addresses and redirects.

## 2. Authorize with PKCE, state and nonce

Discover endpoints at `https://identity.mattediworks.com/.well-known/openid-configuration`. Start Authorization Code with an exact registered redirect, unpredictable `state`, random `nonce`, requested identity scopes and S256 PKCE. Validate state and exchange the one-use code server-side for confidential clients. Never log or persist callback code/state query strings.

Validate RS256 through JWKS, exact issuer/client audience, subject, expiry, issue time and nonce with bounded clock skew. Refresh JWKS on an unknown key ID. An ID token is a signed snapshot, not live session status. No refresh-token/offline-access grant is provided. Send `ui_locales=en` for English; the portal otherwise follows browser preferences with PT-BR fallback.

## 3. Claims and action-scoped authentication

With `openid profile email`, identity fields include `sub`, `email`, `email_verified`, `name`, optional `preferred_username` and `mw_account_type`. Subject is immutable; email and username may change. Unverified email is not mailbox ownership. Identity type is not an application role.

ID tokens also contain `iss`, `aud`, `exp`, `iat`, `nonce`, `auth_time`, `acr` and `amr`. Actual verified proof survives one-use codes. Token issuance and delegation do not reset authentication time.

| Requested ACR | Accepted proof | Achieved context and methods |
| --- | --- | --- |
| `urn:mw:auth:standard` | Password, combined MFA or UV passkey | Actual achieved context below |
| `urn:mw:auth:strong` | Password plus TOTP in one bound flow, or UV passkey | Strong: `pwd`, `otp`; passkey: `webauthn` |
| `urn:mw:webauthn` | UV-verified passkey only | WebAuthn: `webauthn` |

Password alone achieves Standard with `amr=["pwd"]`. Password+TOTP achieves Strong. Passkey achieves WebAuthn and satisfies lower levels. Email+TOTP alone is never Strong. Unknown/unsupported ACR requests fail explicitly; use the published matrix, not lexical comparison. Discovery advertises supported values.

For a sensitive action, request `acr_values=urn:mw:auth:strong&max_age=300`. For a fresh passkey, use `acr_values=urn:mw:webauthn&max_age=0`. Validate the new signed result's assurance, freshness and exact same `(iss,sub)` as the initiating app session. Bind it to a short-lived local action intent and obtain final confirmation for destructive work. Reissuing a token never refreshes `auth_time`.

Migration 016 revokes legacy TOTP-only sessions. Sessions without v1 proof must reauthenticate for v1 authorization. Migration 017 invalidates unfinished legacy codes; migration 019 preserves internal timestamp precision for fresh-auth ordering while JWT time remains seconds. BFF migration 006 binds step-up to the expected subject/WebAuthn ACR. Existing admin high-risk routes remain WebAuthn-only.

Legacy application-wide Strong remains enforced until the consumer's action checks are verified and an operator explicitly retires its gate. See [authentication evidence and cutover](dev_docs/federation-authentication-evidence.md). Deprecated auth/signup columns remain for rollback, not normal new configuration.

## 4. Global registration and application-owned onboarding

Standalone and app-initiated registration use the global emergency switch `IDENTITY_PUBLIC_SIGNUP_ENABLED`. Registration grants no assignment, consent or local role. Private login de-emphasizes signup and explains restricted access; standalone identity creation remains available while globally enabled. Consent never bypasses eligibility.

After validating login, find the local profile by exact `(issuer,subject)` and reuse its local ID/data/permissions. Without a mapping, offer explicit onboarding or independently authenticated linking. Never merge, link or restore by matching email. A recreated MW identity receives a new subject.

## 5. Authorized applications and local account management

Authorized applications represent active consent, not known local accounts; onboarding may have been cancelled. Consent is independent of assignments and token expiry. Missing/revoked/expanded consent requires a prompt; cancellation issues no authorization.

Revoke MW access atomically deactivates consent and invalidates that client's outstanding MW codes/access/delegated tokens. It preserves history, assignments, app profiles, roles, data and independent sessions. It sends no destructive callback and promises no universal app logout. Revoking the last authorized app leaves the MW identity intact.

Later eligible login requests consent again and returns the same issuer/subject. Existing app mappings must reopen the same profile. Eligibility is rechecked at completion and issuance, including assignment removal, deny, app disable and identity deletion.

Manage application account / Delete account in application links navigate to registered static URLs. The app authenticates the local account, applies recent-auth policy and obtains final confirmation. Unlink must preserve a verified alternative login, remove the local MW mapping and invalidate necessary sessions/intents. App-profile deletion erases required local data and permits only explicit new baseline onboarding later. Neither action deletes MW identity.

No central association registry, generation handshake, app-owned lifecycle API or app-session synchronization exists in v1. Apps need not notify Identity of local unlink/deletion. Retire WOI's historical `awaiting_contract` intents in its own repository without replaying them to a nonexistent API.

## 6. Whole-MW-identity deletion

Whole-identity deletion requires recent authentication and explicit confirmation. It disables the identity and revokes central credentials immediately. Candidates come from historical consent/assignments, not verified local account ownership. Receivers delete only currently mapped applicable profiles; detached independent accounts survive.

Migration 018 adds durable operations/deliveries. `IDENTITY_DURABLE_DELETION_ENABLED=false` stages schema with dispatch off; enable only after receiver/backup/recovery checks. When enabled, initiation persists work without outbound HTTP and a bounded worker retries across restarts. Outstanding durable work never falls back to synchronous processing. For a new request while disabled, the legacy three-attempt request-bound path remains; do not claim durable guarantees for that configuration.

The durable payload adds an operation ID to the existing callback:

```json
{
  "id": "evt_example",
  "operation_id": "del_example",
  "type": "identity.user.deleted",
  "sub": "usr_example",
  "application_id": "app_example",
  "client_id": "woi-production",
  "reason": "account_deleted",
  "requested_at": "2026-11-03T12:00:00Z"
}
```

Requests carry a Bearer lifecycle JWT and `X-MW-Identity-Event-ID`. Verify RS256/JWKS, exact issuer/client audience, expiry, subject, event type, and matching JWT `jti`, header and body IDs. Require signed `reason=account_deleted`. Durable signatures additionally bind application ID, operation ID and original request time. Signatures refresh on retry; event/operation IDs and requested time stay fixed. Use the original signed request time for ordering, not refreshed JWT issue time.

Legacy `application_disconnected` must be acknowledged as ignored, never used for deletion or a sign-in cutoff. Missing/unknown/unsigned destructive reasons fail closed. Deploy safe receivers before provider cutover and retain them on rollback.

Legacy receivers must guarantee completed synchronous cleanup before returning an empty `200` or `204`. Nonempty `200` responses must contain a strict JSON envelope: `{"version":1,"outcome":"confirmed_cleanup"}` or `{"version":1,"outcome":"no_current_mapping"}`. The `version` field may be omitted for existing receivers; other fields, duplicate keys, unknown versions, malformed bodies and missing/nonterminal/unknown outcomes never confirm deletion. No-current-mapping acknowledges no current mapped profile without claiming historical erasure. `202` is queued and cannot confirm erasure, regardless of its body. These response rules apply to both durable and request-bound delivery. Duplicate deliveries must be idempotent; receivers fence stale/concurrent login/onboarding. Failed cleanup returns a retryable error.

Missing/changed callbacks, offline apps and retry exhaustion remain recoverable and cannot silently finalize deletion. Internal no-data clients are exempt, distinct from erasure. Final user removal waits for required outcomes; outbox/status records survive its cascade. Identity cannot guarantee removal of app backups, external exports or third-party copies.

Browser progress uses an HttpOnly cookie with a hashed, 24-hour status capability at `/account/deletion`, independent of revoked sessions. An operation ID alone grants no access. Authorized operators have separate status/retry/reconciliation routes. Legacy disabled identities require evidence-backed reconciliation, without invented callbacks or bulk replay.

See the [durable deletion runbook](dev_docs/federation-durable-deletion.md) for leases, outcomes, retry horizon, transport restrictions, retention, APIs and restore requirements.

## 7. Release gates

Back up before additive migrations. Audit each existing client's intended access and Strong behavior before cutover; retain IDs/secrets and exact callbacks. Deploy compatible receivers/provider/consumers in order. Verify disposable real-signature flows, outages/retries, hardware WebAuthn and password+TOTP in browsers. Record unverified checks; unit tests alone do not establish production compliance.

Rollback retains non-destructive consent, TOTP-only rejection, current-mapping protection and outstanding deletion state. Do not reopen deleting identities or drop additive schema. Live-client audit, external consumer adoption and signed-off production recovery/rollout remain operator release checkpoints.
