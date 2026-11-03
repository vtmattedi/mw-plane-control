# MW Identity Federation Contract v1.0

> **Status:** Architecture contract freeze — normative target; implementation compliance pending  
> **Owner:** MW Identity (`vtmattedi/mw-plane-control`)  
> **Date:** 2026-11-03  
> **Version:** 1.0.0  
> **Consumers:** Working On It (WOI), Stackport, and future MW applications  
> **Source of truth:** This contract governs intended behavior. Existing implementation and integration guides must adapt to it; current code is not automatically considered compliant.

## 1. Scope and architectural ownership

MW Identity is the **identity provider (IdP)**. It owns:

- MW identities and immutable subject identifiers (`iss` + `sub`).
- Credentials, authentication methods, and central sessions.
- Registration and administration of OAuth/OIDC clients.
- Application access eligibility and per-identity assignments.
- OIDC claims, assurance evidence, and step-up authentication.
- User consent and revocation of MW-issued credentials.
- Coordination of **whole-MW-identity deletion**, as defined in Section 10.

Each consuming **application** owns:

- Its application-local accounts, profiles, roles, memberships, and data.
- Its mapping between `(issuer, subject)` and local user IDs.
- Any independent login methods and application-issued sessions.
- Account linking, unlinking, local reauthentication, and application-account deletion.
- Authorization for application-specific operations.

### 1.1 Distinct concepts

| Concept | Meaning | Owner |
| --- | --- | --- |
| Identity | The person and their credentials | MW Identity |
| OAuth client | An administrator-approved application integration | MW Identity |
| Access eligibility | Whether a subject can authenticate for an application | MW Identity |
| Authentication assurance | How and when the subject authenticated | MW Identity, via signed evidence |
| Consent | Permission to issue requested identity claims and MW tokens to a client | MW Identity |
| Account association | `(iss, sub)` mapped to an application-local profile | Application |
| App authorization | Roles, capabilities, and business permissions | Application |
| App-account deletion | Removal of an application's account and data | Application |
| Whole-identity deletion | Removal of an MW identity and coordinated required downstream cleanup | MW Identity + apps |

**Non-negotiable invariants:**

1. Consent is **not** proof of the existence of an application account. A user can authorize an application and abandon onboarding.
2. Revoking consent **must not** delete or detach an application account.
3. An MW identity may exist with **zero** authorized applications.
4. An application must never automatically link, merge, or restore accounts by matching email.
5. Application-local roles and data are never inferred from MW authentication or MW assignment alone.
6. Terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** define normative requirements.

## 2. Application registration

An authorized MW administrator must register and approve an application before it can use MW Identity. Application self-registration is outside v1.0.

Each application registration contains:

| Property | Contract |
| --- | --- |
| `client_id` | Unique, stable public identifier for OAuth/OIDC |
| `client_secret` | Generated and shown once **only for confidential clients**; securely rotated |
| OAuth client type | `confidential` or `public` |
| Application access | `public` or `private` |
| Enabled / archived state | Controls whether credentials may be issued |
| Redirect URIs | Exact registered HTTPS values |
| Post-logout redirect URIs | Optional exact registered values |
| Scopes, claims, audiences | Explicitly permitted values |
| `delete_callback_uri` | Required for ordinary applications under whole-identity deletion policy |
| `account_management_uri` | Optional validated HTTPS account-management navigation |
| `account_deletion_uri` | Optional validated HTTPS account-deletion navigation |

A **confidential** OAuth client can protect a secret (for example, a backend-based WOI deployment). MW issues a client secret, stores only a secure verifier/hash, and authenticates the client at token exchange. A **public** OAuth client cannot protect a secret (for example, a browser-only or native application); MW does not issue it a secret. Both use PKCE.

**Do not confuse OAuth `public` client type with public application access.** A publicly accessible application may use a confidential OAuth client.

An application-wide `standard`/`strong` policy and per-application `allow_public_signup` switch are **not** part of this frozen registration model.

### 2.1 Trusted MW infrastructure

Trusted infrastructure clients, such as narrowly scoped components behind the MW BFF, may require privileged audiences or other exceptional handling. They must be provisioned through a separate controlled and audited infrastructure workflow, **not** through an ordinary `internal` toggle available to all applications.

A restricted ordinary app is **private**, not infrastructure. Infrastructure classification must not be used to bypass consent or deletion obligations for apps that own local user accounts.

## 3. Public and private access

The **access policy** answers *who is eligible to authenticate for this application?*

| Policy | Default | Per-user assignment |
| --- | --- | --- |
| **Public** | An eligible enabled MW identity may authorize | Explicit deny overrides the default |
| **Private** | Denied | Explicit allow is required; explicit deny denies |

Eligibility is evaluated **server-side** during authorization and rechecked before credential issuance. Creating an identity or granting consent does not bypass eligibility. A private assignment grants eligibility to authenticate, not a local application role or administrator privilege.

**Superadmin boundary:** MW superadmin status must not silently bypass ordinary private-app allowlists or explicit denials. Any emergency override needs a distinct, narrow, audited break-glass contract; it is not part of normal OIDC federation.

### 3.1 Identity registration is global

The MW public portal allows new people to create MW identities independently of applications. Application registration has **no separate signup-enable flag**.

- A new MW user may create an identity even when attempting to reach a private application.
- That identity still cannot access the private application without an explicit assignment.
- During a private-app-initiated login flow, the UI **should hide or de-emphasize signup** and explain restricted access. This is a UX decision only, never the authorization check.
- Existing identities may use public applications without needing a second MW registration.

MW may distinguish `public` and `workforce` **identity types**; that is separate from public/private **application access**. A publicly registered identity may be allowed into an ordinary private app if expressly assigned. Privileged MW admin audiences remain separately restricted.

An unverified email is not proof of mailbox ownership; even a verified email is not a safe automatic account-linking key.

## 4. OIDC login and application onboarding

All ordinary clients must use **Authorization Code + S256 PKCE** with an exact redirect URI, unpredictable `state`, and `nonce`. Clients must validate `state`, exchange the one-time code, and verify the signed ID token including `iss`, `aud`, `exp`, `iat`, `nonce`, and `sub` (with bounded clock-skew handling). A confidential client's secret remains server-side.

### 4.1 Flow

1. The user selects **Continue with MattediWorks** in the application.
2. The application starts OIDC authorization for its registered `client_id` and requested scopes.
3. MW authenticates the person, enforces access eligibility, and requests missing consent.
4. MW returns an authorization code; the application validates and exchanges it.
5. The application validates the ID token and searches for a local account keyed by **exact** `(iss, sub)`.
6. When a mapping exists, the application signs into that **same** local profile, retaining its local ID, data, and permissions.
7. When no mapping exists, the application presents explicit onboarding or an independently authenticated linking workflow. Neither MW nor the application silently merges profiles by email.

MW login does **not** grant local administrator roles. The application owns its authorization model and may also maintain independent local login methods.

## 5. Authentication assurance is token-based, not app-based

**Freeze decision:** `standard` and `strong` are assurance requirements for **individual actions/sessions**, **not permanent application properties**. MW attests to the authentication performed. Each application decides what proof and freshness a particular operation requires.

### 5.1 Assurance levels

| Requested ACR | Satisfying authentication evidence | Does not satisfy |
| --- | --- | --- |
| `urn:mw:auth:standard` | Password or a verified passkey | Unverified identity hint |
| `urn:mw:auth:strong` | Verified passkey **or** password **plus** TOTP as a combined authentication flow | Email + TOTP **alone** |
| `urn:mw:webauthn` | WebAuthn/passkey ceremony with required user verification | Password + TOTP without a passkey |

A stronger proof may satisfy a lower requirement. An explicit passkey requirement is **not** met by password + TOTP. If a requested ACR cannot be satisfied, MW must fail explicitly, without silently lowering the requested assurance.

**Important implementation requirement:** TOTP-only login is not password-plus-TOTP and must not be asserted as `strong`. The current TOTP-only fallback requires redesign before claiming conformance.

### 5.2 ID token contract

MW's signed OIDC ID token must accurately include:

- `acr`: the achieved authentication context, using the published MW ACR values.
- `amr`: truthful authentication methods, using documented method names (for example `pwd`, `otp`, `mfa` where applicable, and a documented passkey identifier).
- `auth_time`: the actual authentication event time, not the token's issue time.
- Standard protocol claims including `iss`, `sub`, `aud`, `iat`, `exp`, and the requested `nonce`.

Illustrative target payload (values are examples, not currently deployed tokens):

```json
{
  "iss": "https://identity.mattediworks.com",
  "sub": "usr_example",
  "aud": "woi-production",
  "iat": 1793742000,
  "exp": 1793742300,
  "auth_time": 1793741900,
  "acr": "urn:mw:auth:strong",
  "amr": ["pwd", "otp"],
  "nonce": "request-bound-random-nonce"
}
```

MW must publish supported ACRs and their precise satisfaction rules. The additional `urn:mw:auth:standard` and `urn:mw:auth:strong` values are **new normative targets**; the current provider already recognizes `urn:mw:webauthn` for explicit passkey step-up. Client code must evaluate assurance using the published rules, not string ordering.

### 5.3 Step-up

An app may permit ordinary password login, then require stronger or fresher proof for a sensitive action.

For example, WOI can request a recently achieved Strong assurance level:

```text
acr_values=urn:mw:auth:strong
max_age=300
```

For an action that mandates a fresh passkey ceremony:

```text
acr_values=urn:mw:webauthn
max_age=0
```

`max_age=0` requires fresh authentication. When an existing authentication proof already meets the requested level and nonzero age limit, it may be reused. **Reissuing an ID token does not reset `auth_time`.**

The application must validate the **new signed** step-up result, required achieved ACR, `auth_time`, nonce/state, and **exact same `(iss, sub)`** as the initiating app account. The operation must be bound to the original application session, a short-lived action intent, and final user confirmation for destructive actions.

An ID token is a **signed snapshot**, not a live MW session status. Apps may store validated authentication provenance in their own sessions, but they must require step-up when assurance/freshness is missing. Accounts with no MW mapping must use an app-local safe reauthentication path instead.

## 6. Consent, revocation and reauthorization

Consent authorizes MW to issue requested identity information and credentials to an eligible registered client. It is **not** an account link, role assignment, or deletion instruction.

- Missing, revoked, or expanded consent requires a consent prompt.
- Cancelled consent does not issue the authorization.
- Identity's **Authorized applications** screen represents **active consent**, not verified external application-account existence.

### 6.1 Revoking MW access

From the Identity portal, **Revoke MW access**:

1. Deactivates the consent for that subject/client and invalidates applicable MW authorization codes and MW-issued access/delegated tokens.
2. Retains required consent history for audit and whole-identity deletion discovery.
3. **Does not** delete local application data, unlink `(iss, sub)`, remove local roles, or revoke independently issued app sessions.
4. Sends **no destructive deletion callback**.

The UI must explain that an existing app-issued session might remain active. This operation must not be labeled *Delete account* or *Unlink local account*.

### 6.2 Reauthorizing

A user who selects **Continue with MW** after revocation must be offered consent again, subject to **current** access eligibility. On successful authorization, an existing local profile mapped to the same `(iss, sub)` must be reused, with no duplicate onboarding.

Revoking the last consenting application **must not** delete or orphan the MW identity. If a previously authorized private-app user is no longer assigned, they cannot reauthorize until access is restored; their app profile is not implicitly deleted.

## 7. Local account linking and unlinking

**Unlink** belongs to the application because only it knows whether the retained account has another verified sign-in method.

- To link an existing local account, the user must independently authenticate to that account and complete a fresh MW flow proving control of the intended MW subject.
- The application must not unlink when doing so would leave a retained active profile with no usable verified alternative login.
- Before unlinking, offer setup/verification of another login method or an authenticated local account-deletion workflow.
- After unlink, the application removes the local MW mapping and invalidates the necessary local sessions/intents, but **retains** the independent account and its data.
- Old MW consent may remain in Identity and may be revoked **separately** by the user.
- Future MW login cannot silently relink a detached profile by email or previously granted consent. Explicit linking proof is required; otherwise the app may offer **new** onboarding.

No central account-association registry, generation handshake, or application-owned lifecycle API is required for the v1.0 consent and unlink semantics.

## 8. Application management navigation

MW Identity should store optional validated HTTPS application URLs:

```text
account_management_uri
account_deletion_uri
```

These are **navigation targets only**. They may be shown in the Authorized Applications and historical application views:

- **Open application**
- **Manage application account**
- **Delete account in application**
- **Revoke MW access** — a separate Identity-owned consent action

The navigation URL must not contain bearer tokens, user IDs, secret proof, or an authorization to delete. An optional `?intent=delete` is just a UI hint. The application must independently authenticate the correct local account, validate its recent authentication policy, show deletion consequences, and require explicit final confirmation.

The portal must not claim that an external account exists or was deleted based only on consent records.

## 9. Application-account deletion

Local account deletion is completed by the application. It must not remove the MW identity. The app invalidates affected sessions, credentials, link intents, memberships/data as appropriate, and performs its own safe cleanup.

For an MW-linked user, recent MW step-up and an exact subject match may be required. A local-only user uses an equivalent app-local reauthentication process.

After app-account deletion, a later permitted MW authorization may lead to **fresh onboarding**. It must not restore the old profile, roles, or data. Provider consent might still be active until separately revoked; this does not mean the old app account still exists.

Applications do not have to notify MW of app-account deletion under v1.0.

## 10. Whole-MW-identity deletion

The existing MW product policy is retained: **deleting the entire MW identity coordinates required cleanup of applicable profiles in registered non-infrastructure applications.** This is separate from consent revocation and app-local deletion.

MW Identity must:

1. Require appropriate recent authentication and an explicit whole-account confirmation.
2. Atomically mark the identity as disabled/deleting, prevent further authorization and onboarding, and revoke central credentials/sessions.
3. Persist a deletion operation, stable event IDs, and a snapshot of known app obligations (derived from historical consent/assignments, **not** an authoritative app-account registry).
4. Deliver signed events durably, with background retries, crash recovery, idempotent identifiers, and per-app progress visible to authorized operators/users.
5. Use the destructive event `identity.user.deleted` only with the signed `reason=account_deleted`, exact audience, issuer, subject, event ID, and bounded expiry.
6. Never treat missing callbacks, merely queued work, or historical consent as proof of erasure; distinguish pending, failed, completed, and no-current-association outcomes.
7. Complete final deletion according to the documented policy after required outcomes and retain minimal permitted replay/operational records.

Each application receiver must verify the event's signature/JWKS, issuer, client audience, subject, expiry, event ID coherence between header/body/token, and the **signed** reason. Only `account_deleted` may trigger destructive cleanup. A legacy `application_disconnected` event must be ignored as **non-destructive**.

The receiver must delete only a **currently mapped** applicable profile, never a previously unlinked independent account. It must prevent stale login handoffs or concurrent onboarding from recreating a deleted profile. Processing must be idempotent; a success acknowledgment is returned only after required local cleanup succeeds. When no current mapping exists, the receiver may acknowledge that there is nothing to delete without asserting that historical data was erased.

A failed or unreachable application must leave a recoverable pending deletion operation. Identity must not claim it can guarantee removal of independent app backups or historical exports.

## 11. Session and security boundaries

- MW browser sessions, OAuth access tokens, OIDC ID tokens and application-issued sessions are separate credentials.
- Revoking consent prevents further MW authorization and invalidates applicable MW-issued credentials, but **does not promise universal logout** of independent app sessions.
- ID token expiry or a new ID token does not prove live state or change the original authentication time.
- No refresh-token grant, `offline_access` right, central app-session epoch, or back-channel logout guarantee is introduced by v1.0. Such features require separate versioned contracts.
- Apps retain responsibility for their own sessions, CSRF protection, authorization, and authentication-provenance tracking.
- Production redirects and callbacks require strict URI validation, HTTPS, bounded request/response sizes and timeouts, no unsafe callback redirects, and safe handling of key rotation.
- Sensitive operations must be auditable without exposing raw credentials, tokens, TOTP secrets, or recovery material.

## 12. Acceptance scenarios

| ID | Scenario | Expected result |
| --- | --- | --- |
| A01 | Person visits MW portal without an app | Can create MW identity |
| A02 | Unassigned person opens Public app | Eligible for OIDC and consent; local onboarding remains app-owned |
| A03 | Unassigned person opens Private app | Access denied even if user exists or once consented |
| A04 | Assigned person opens Private app | Can authorize but gets no implicit local admin role |
| A05 | Explicitly denied person opens Public app | Access denied |
| A06 | MW superadmin explicitly denied from ordinary app | No silent bypass |
| A07 | Returning MW user opens linked app | Same local profile, data, and roles |
| A08 | Consent accepted, app onboarding cancelled | No app account inferred by MW |
| A09 | User revokes consent for sole authorized app | MW identity and app account remain intact |
| A10 | User reauthorizes that app | Consent requested again; same linked local profile used |
| A11 | App user tries to unlink the only login | Blocked until alternative verified login, or offered deletion |
| A12 | User unlinks after verifying app-local password | Local account survives; future MW sign-in does not silently relink |
| A13 | Identity links to an app deletion page | App reauthenticates and confirms; MW identity survives |
| A14 | Password-only login | Standard evidence, truthful `amr` and `auth_time` |
| A15 | App requests Strong with password-only session | Step-up required, no silent downgrade |
| A16 | Password + TOTP completed | Strong evidence; both factors reflected in `amr` |
| A17 | Email + TOTP without password | Never asserted or accepted as Strong |
| A18 | Verified passkey login | Satisfies Strong and explicit WebAuthn requirement |
| A19 | App requests fresh passkey with `max_age=0` | New ceremony and same subject/action-bound validation |
| A20 | Stale or wrong-subject step-up result | Operation denied |
| A21 | Consent revoked mid-authorization | Stale authorization/code exchange rejected |
| A22 | Private access removed mid-authorization | Authorization cannot complete |
| A23 | Whole-identity deletion with app offline | Identity disabled, durable delivery pending and retryable |
| A24 | Duplicate/delayed deletion events | Idempotent; detached profiles never erased |
| A25 | Onboarding races with identity deletion | Cannot recreate deleted profile |
| A26 | OAuth public client | No secret; PKCE required |
| A27 | Confidential OAuth client | Client secret verified; rotation supported |
| A28 | Infrastructure client | Exceptions require restricted provisioning and audit |

Tests must include provider integration tests and cross-service scenarios with disposable state, real signing keys, simulated outages, duplicated events, concurrent requests, and browser-based authentication step-up.

## 13. Known implementation differences — engineering backlog

**This section is informative, not a relaxation of the contract.** Current code reviewed in `vtmattedi/mw-plane-control` on 2026-11-03.

| Current behavior | Required change |
| --- | --- |
| Applications store `authentication_policy=standard/strong` and enforce it globally | Retire this requirement and use action-scoped ACR + signed evidence |
| Strong currently accepts WebAuthn or **TOTP-only** authentication | Implement combined password + TOTP and prohibit TOTP-only as Strong |
| ID tokens do not currently contain `acr`; TOTP may be represented as `unknown` in `amr` | Issue truthful `acr`, `amr`, `auth_time`; publish supported ACRs |
| `allow_public_signup` can bypass `default_deny` for unassigned users | Remove per-app signup setting and eligibility coupling; preserve global registration |
| MW superadmin bypasses ordinary application access restrictions | Remove implicit bypass or introduce a separate explicitly documented break-glass mechanism |
| `internal` appears among normal app configuration | Move infrastructure clients to a privileged provisioning workflow |
| Application management/deletion navigation not registered | Add URL metadata, validations, and portal links |
| Whole-identity deletion retries callbacks inside the request | Add persistent outbox, stable delivery IDs, worker, and recoverable status |
| Authorized-app view is consent-based | Keep it, but make wording explicitly about authorization, not known accounts |
| WOI has pending `awaiting_contract` lifecycle-operation receipts | Retire/reconcile safely; do not replay against an API this contract does not define |

**Release order:** Freeze this document → reconcile existing integration guide and rollout strategy → implement provider changes → adapt applications → pass cross-service acceptance tests → declare compliance. Do not advertise new assurance claims, URLs, or guarantees until deployed and verified.

## 14. Versioning and freeze rule

Any change affecting **eligibility**, **ACR semantics**, **consent versus unlink/delete**, **deletion callback destructiveness**, or **client authentication** requires an explicit versioned contract amendment and compatibility plan. Undocumented implementation behavior is not an implicit contract change.

### References

- [Current MW Identity application integration guide](https://github.com/vtmattedi/mw-plane-control/blob/main/docs/MW-Identity-application-integration.md)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [RFC 8176 — Authentication Method Reference Values](https://www.rfc-editor.org/rfc/rfc8176)

---

**Freeze statement:** *Identity owns identity, eligibility, consent and authentication evidence. Applications own local accounts and action authorization. Consent revocation is never account deletion. Assurance is expressed and verified per action, not as a fixed application category.*
