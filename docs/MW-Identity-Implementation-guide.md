# MW Identity — Implementation Guide

Federation v1 now follows [the integration guide](MW-Identity-application-integration.md) and [evidence record](dev_docs/MW-Identity-Federation-v1-Compliance.md). Legacy app-wide Strong and signup-based access below remain only for reviewed migration; TOTP-only is no longer Strong.

**Component:** `mw-identity`  
**Role:** central human identity, authentication, application registry, authorization issuer, token exchange, introspection, revocation, and recovery  
**Version baseline:** MW Control Plane v1.0

---

## 1. Mission

MW Identity is the authoritative human identity service for the MattediWorks control plane.

The stable human principal is:

```text
issuer + sub
```

Identity owns authentication and identity-level authorization.

It must not absorb the business-user models of consuming applications.

---

## 2. Core responsibilities

MW Identity owns:

- Identity users;
- stable subjects;
- global superadmin bootstrap;
- password verification;
- Identity browser sessions;
- Identity CSRF;
- OIDC discovery;
- JWKS;
- RSA signing keys;
- application/client registry;
- Authorization Code + PKCE;
- access and ID-token issuance;
- application assignments;
- Identity admin permissions;
- WebAuthn;
- TOTP fallback;
- assurance and `auth_time`;
- explicit WebAuthn ACR handling;
- delegated token exchange;
- authenticated introspection;
- session/token revocation;
- manual recovery;
- Identity audit;
- key rotation;
- audit retention and operational cleanup.

---

## 3. Identity does not own

Identity does not own:

- MW Mail projects;
- MW OAuth projects;
- application-local domain users;
- application-local business roles unless explicitly modeled through assignments;
- browser BFF sessions;
- BFF CSRF;
- downstream business logic;
- downstream project API keys;
- OAuth callback HMAC secrets;
- MW Mail sender API keys.

Existing applications keep their own user tables and their own `issuer + subject -> local_user_id` mapping.

---

## 4. Stable subject model

Identity user IDs are opaque stable subjects:

```text
usr_...
```

OIDC:

```text
sub = Identity user ID
```

Never use email as the permanent identity key.

Email may change.

Application-local account linking belongs to the relying application.

---

## 5. Local authentication

Identity supports local password authentication.

Rules:

- passwords are securely hashed;
- passwords are never logged;
- passwords are never returned;
- passwords are never sent to BFF/downstream services;
- login uses Identity-side pre-auth CSRF;
- login abuse controls remain bounded;
- recovery-pending users cannot bypass recovery through normal password login.

Identity browser sessions are opaque and server-side.

---

## 6. OIDC discovery

Public endpoints:

```text
GET /.well-known/openid-configuration
GET /.well-known/jwks.json
```

Canonical production issuer:

```text
https://identity.mattediworks.com
```

The public issuer is protocol identity, not an internal Docker URL.

Internal callers may use:

```text
http://mw-identity:7100
```

for transport while still validating the canonical issuer.

---

## 7. Signing-key architecture

ID tokens use:

```text
RS256
```

Signing keys:

```text
RSA
use=sig
unique kid
```

Private signing material is encrypted in SQLite.

Configuration:

```env
IDENTITY_SIGNING_KEYS=1:<base64url-32-byte-key>
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=1
```

This keyring encrypts private signing keys at rest. It is not itself the OIDC RSA signing key.

Multiple encryption-key versions may coexist to support keyring rotation.

Identity must fail unsafe startup when required keyring material is absent or invalid.

---

## 8. Signing-key rotation

Operator command:

```powershell
docker compose exec mw-identity /app/rotate-signing-key
```

Correct flow:

```text
generate RSA key
    ↓
new kid
    ↓
new active signing key
    ↓
old signing key retired
    ↓
old public JWK remains available temporarily
```

Do not remove the previous public JWK immediately.

Tokens signed immediately before rotation must remain verifiable for their valid lifetime.

Step 13:

```env
IDENTITY_SIGNING_RETIRED_RETENTION_HOURS=72
```

Production validation must reject an unsafe retention period.

---

## 9. Application registry

Applications represent OIDC clients and resource servers.

Important fields:

```text
id
name
client_id
client_secret_hash
application_type
authentication_policy
enabled/archive state
redirect URIs
post-logout redirect URIs
allowed scopes
allowed claims
allowed audiences
timestamps
```

Types:

```text
confidential
public
```

Authentication policies:

```text
standard
strong
```

`client_id` is immutable.

Application type is immutable.

Redirect URI matching is exact.

Do not introduce wildcard redirect URIs.

---

## 10. Client-secret lifecycle

Confidential application creation returns a raw secret once.

Identity stores only its hash.

Rotation:

```text
authorized recent admin operation
    ↓
new random secret
    ↓
persist new hash
    ↓
return raw secret once
    ↓
old secret invalid immediately
```

Do not make client secrets retrievable later.

---

## 11. Application lifecycle

```text
enabled <-> disabled
enabled/disabled -> archived
```

Archived applications are terminal.

Hard delete is intentionally avoided for trust/audit continuity.

Disabling an application is the emergency authorization kill switch.

---

## 12. Assignments

Assignments answer:

> May this Identity subject use this application, and in which coarse role?

Key:

```text
application_id + user_id + role
```

Roles:

```text
user
admin
superadmin
```

Ordinary OIDC authorization requires assignment.

Bootstrap exception:

```text
global Identity superadmin
+
identity.admin scope
+
mw-identity-admin audience
```

This exception must stay narrow.

Never turn global superadmin into universal implicit assignment.

---

## 13. Identity admin permission model

Representative permissions:

```text
identity.applications.read
identity.applications.write
identity.applications.rotate_secret

identity.users.read

identity.assignments.read
identity.assignments.write

identity.audit.read
```

A normal admin must not gain assignment-write permission unless deliberately authorized.

For sensitive mutations:

1. determine the actor;
2. evaluate required permission;
3. evaluate fresh-WebAuthn requirement;
4. perform mutation;
5. append safe audit.

Permission evaluation should happen before assurance signaling where needed so hidden privileged operations are not leaked to unauthorized users.

---

## 14. Authorization Code + PKCE

Identity supports:

```text
GET  /oauth/authorize
POST /oauth/token
GET  /oauth/userinfo
```

Security contract:

```text
Authorization Code only
PKCE mandatory
S256 only
state required by MW policy
exact redirect URI
one-time authorization code
confidential client authentication
opaque access token
RS256 ID token
```

Authorization codes are:

- random/high entropy;
- short-lived;
- stored only as hashes;
- consumed atomically;
- single-use.

---

## 15. Baseline lifetimes

```text
authorization code: 5 minutes
access token:       60 minutes
ID token:            5 minutes
delegated token:     5 minutes
```

Do not lengthen security-token TTLs casually.

---

## 16. ID-token claims

Trusted claims include:

```text
iss
sub
aud
exp
iat
auth_time
nonce when requested
profile claims when scopes permit
authentication method/assurance context as implemented
```

`sub` is the stable Identity subject.

Do not put secrets or application-domain objects into ID tokens.

Current application-facing identity claims are documented in [MW-Identity application integration](MW-Identity-application-integration.md). In particular, relying applications key users by `sub`, receive standard `email_verified`, and can distinguish `public` from `workforce` through `mw_account_type`.

---

## 17. Access-token model

Identity access tokens are opaque random bearer values.

Persist hashes where token lookup is needed.

Downstream services should not parse them as JWTs.

Opaque resource/delegated tokens are validated through authenticated introspection.

---

## 18. WebAuthn ownership

WebAuthn belongs to Identity.

BFF does not verify WebAuthn assertions.

Production:

```env
IDENTITY_WEBAUTHN_RP_ID=identity.mattediworks.com
IDENTITY_WEBAUTHN_RP_NAME=MW Identity
IDENTITY_WEBAUTHN_ORIGINS=https://identity.mattediworks.com

IDENTITY_WEBAUTHN_CREDENTIAL_KEYS=1:<base64url-32-byte-key>
IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION=1

IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES=5
```

No wildcard WebAuthn origins.

A deliberate localhost RP/origin may be used for local browser testing.

---

## 19. WebAuthn credential policy

Baseline:

```text
resident/discoverable = required
userVerification      = required
attestation           = none
```

Each user has a stable random opaque WebAuthn user handle.

Do not derive the user handle from email.

Multiple credentials are supported.

Credential IDs must remain unique.

Serialized credential state is encrypted at rest with the dedicated versioned WebAuthn keyring.

---

## 20. WebAuthn ceremonies

Ceremony state must be:

```text
random
short lived
single use
purpose bound
user/context bound
atomically consumed
```

Validate:

```text
challenge
origin
RP ID
user verification
credential ownership
purpose
expiry
single use
```

Do not log:

```text
raw challenge
full authenticator response
serialized credential blob
browser biometric/PIN data
```

---

## 21. WebAuthn enrollment hardening

Current policy:

- first passkey may use the supported bootstrap/migration path;
- after an account has an established passkey trust set, adding another requires recent WebAuthn;
- normal credential removal requires recent WebAuthn;
- the final passkey cannot be removed through the normal removal flow;
- catastrophic trust replacement uses operator recovery.

---

## 22. TOTP fallback

Configuration:

```env
IDENTITY_TOTP_SECRET_KEYS=1:<base64url-32-byte-key>
IDENTITY_TOTP_SECRET_ACTIVE_KEY_VERSION=1
IDENTITY_TOTP_ISSUER=MW Identity
```

Baseline:

```text
RFC 6238-style
SHA-1
6 digits
30-second period
validation window ±1 step
```

Secret rules:

- random 20-byte secret;
- Base32 representation for enrollment;
- encrypted at rest;
- never logged.

Persist the last accepted timestep and reject replay of the same/older accepted timestep.

TOTP is fallback authentication, not a replacement for explicit WebAuthn step-up.

---

## 23. Authentication policies

Application policy:

```text
standard
strong
```

`standard` can accept compatible legacy/password assurance.

`strong` requires strong authentication.

Normal strong authentication accepts:

```text
WebAuthn
or
TOTP fallback
```

Explicit WebAuthn ACR accepts only WebAuthn.

---

## 24. ACR and freshness

Supported explicit ACR:

```text
urn:mw:webauthn
```

Caller may also supply bounded positive `max_age`.

Identity evaluates:

```text
session auth method
auth_time
required ACR
max_age
```

If the current session is insufficient, redirect to the required authentication mode.

TOTP must never satisfy:

```text
acr_values=urn:mw:webauthn
```

---

## 25. `auth_time` and authentication method

Identity sessions persist trusted authentication time.

OIDC/delegated tokens propagate the correct authentication context.

Do not substitute token issuance time for human authentication time.

Downstream/BFF assurance decisions rely on trusted `auth_time` and authentication method (`amr`/equivalent context).

---

## 26. Token exchange

Identity supports narrow on-behalf-of delegation.

Conceptual request:

```text
grant_type=urn:ietf:params:oauth:grant-type:token-exchange
subject_token=<BFF Identity subject access token>
subject_token_type=urn:ietf:params:oauth:token-type:access_token
requested_token_type=urn:ietf:params:oauth:token-type:access_token
audience=<resource>
scope=<service scopes>
```

Validate:

- caller is confidential/authenticated;
- subject token is active;
- subject token belongs to the expected source-client context;
- user is enabled;
- requested audience is allowed;
- requested scopes are allowed;
- requested scopes are within subject authority;
- requested scopes are service scopes;
- Identity-only scopes are rejected.

---

## 27. Delegated token model

Delegated token:

```text
opaque
short lived
same human sub
exact audience
narrow scopes
auth_time retained
authentication method retained
```

Config:

```env
IDENTITY_DELEGATED_TOKEN_TTL_MINUTES=5
```

Effective expiry must not exceed the subject token's remaining validity.

Do not switch delegated tokens to JWTs merely for convenience; opaque token + introspection is the v1.0 contract.

---

## 28. Authenticated introspection

Endpoint:

```text
POST /oauth/introspect
```

The resource server authenticates with its own confidential Identity client.

Critical invariant:

```text
token.aud == introspecting_application.client_id
```

This prevents one service from introspecting/using authority minted for another service.

---

## 29. Resource-server registration

MW OAuth:

```text
client_id = mw-oauth
aud       = mw-oauth
```

MW Mail:

```text
client_id = mw-mail
aud       = mw-mail
```

Future example:

```text
client_id = mw-storage
aud       = mw-storage
```

If the current registry requires `openid` metadata for every application, a pure resource server may carry `openid` only as registry metadata. It must not receive `openid` as a delegated service permission.

---

## 30. Adding a new delegated scope

Example:

```text
mail.domains.verify
```

Identity-side checklist:

1. Add/allow the scope in the relevant BFF application registration.
2. Add it to the service/resource registration if registry semantics require it.
3. Ensure token exchange recognizes it as a valid service scope.
4. Keep it outside the Identity-only scope set.
5. Test allowed exchange.
6. Test disallowed exchange.
7. Keep downstream service scope enforcement.

Identity issuing a scope does not replace downstream authorization.

---

## 31. Adding a new service

For `mw-storage`:

1. Register confidential resource application:

```text
client_id=mw-storage
audience=mw-storage
```

2. Define service scopes:

```text
storage.objects.read
storage.objects.write
storage.credentials.rotate
```

3. Securely deliver its one-time introspection secret to MW Storage runtime configuration.

4. Add `mw-storage` to BFF application's allowed audiences.

5. Add required `storage.*` scopes to BFF application's allowed scopes.

6. Test:

```text
correct audience -> active
wrong audience   -> inactive/denied
revoked token    -> inactive
disabled app     -> denied
Identity outage  -> service fails closed
```

---

## 32. Account linking contract

Identity assignments do not identify a consuming application's local user row.

Relying applications should maintain an equivalent mapping:

```sql
external_identities (
    local_user_id,
    issuer,
    subject,
    linked_at,
    last_login_at,
    UNIQUE (issuer, subject),
    UNIQUE (local_user_id, issuer)
)
```

Preferred linking:

```text
authenticated local account
    ↓
explicit Link MW Identity
    ↓
OIDC
    ↓
validated issuer + sub
    ↓
store local mapping
```

Do not auto-link based only on matching email.

Unlinking removes the local mapping; it does not delete the Identity user.

---

## 33. Revocation

User compromise:

```text
revoke Identity sessions
revoke authorization codes
revoke access tokens
revoke delegated tokens
```

The Identity admin surface includes user session/token revocation.

Operator recovery also performs broad revocation.

Application compromise:

```text
disable application
rotate client secret
audit/investigate
```

Never keep a known revoked token valid merely because a local cache has not expired.

---

## 34. Manual recovery

There is no email recovery.

Start:

```powershell
docker compose --env-file .env.production run --rm `
  --entrypoint /app/recover-identity `
  mw-identity `
  --email admin@example.com
```

Start behavior:

```text
recovery_pending=true
revoke sessions/codes/tokens
generate random mwrec_ capability
store only capability hash
short TTL
print raw capability once
```

Recovery uses the token through a URL fragment.

Successful recovery establishes replacement WebAuthn trust, removes old TOTP fallback, and requires a fresh normal WebAuthn login before privileged work.

Cancel:

```powershell
docker compose --env-file .env.production run --rm `
  --entrypoint /app/recover-identity `
  mw-identity `
  --cancel `
  --email admin@example.com
```

Cancellation invalidates the recovery capability but does not resurrect revoked sessions.

---

## 35. Audit

Audit should record safe events for:

- user creation;
- login success/failure;
- logout;
- application create/update/lifecycle;
- secret rotation;
- assignments;
- WebAuthn;
- TOTP;
- token exchange;
- revocation;
- recovery;
- privileged admin mutations.

Never store:

```text
password
raw session token
raw CSRF
authorization code
raw access/delegated token
raw client secret
TOTP secret
recovery capability
raw WebAuthn challenge
full assertion/credential payload
```

Step 13 baseline:

```env
IDENTITY_AUDIT_RETENTION_DAYS=180
```

---

## 36. SQLite persistence

Identity owns its own SQLite database.

Required:

```text
PRAGMA journal_mode = WAL
PRAGMA foreign_keys = ON
PRAGMA busy_timeout = 5000
```

Migration rules:

- ordered embedded migrations;
- do not rewrite already-applied production migrations;
- add a new migration for semantic changes;
- use atomic conditional writes for one-time capabilities/ceremonies;
- store hashes instead of raw capability tokens where possible.

---

## 37. Backup

Identity supports SQLite-aware online snapshots used by Step 13.

Do not copy a live `.db` while ignoring WAL state.

Database backups intentionally exclude keyrings.

A restored `identity.db` cannot fully recover encrypted security state if these are lost:

```text
IDENTITY_SIGNING_KEYS
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS
IDENTITY_TOTP_SECRET_KEYS
```

Store keyrings separately from DB backup archives.

---

## 38. Production config validation

Fail startup/production validation for unsafe mandatory configuration.

Validate at least:

- canonical issuer;
- signing keyring and active version;
- WebAuthn RP/origins;
- WebAuthn keyring;
- TOTP keyring when enabled;
- valid TTLs;
- safe retired signing-key retention;
- valid DB/runtime configuration;
- no obvious placeholder production secrets.

Critical production secrets must not silently fall back to development defaults.

---

## 39. Health and readiness

Expose:

```text
/health
/ready
```

`/health` means process alive.

`/ready` means normal service responsibilities, including persistence, are ready.

Do not leak database paths, credentials or stack traces through readiness.

---

## 40. Request IDs and logging

Propagate:

```text
X-Request-ID
```

Safe logs may include:

```text
request_id
method
path
status
duration
safe sub
safe app ID
safe event type
```

Never log sensitive auth query/body data.

---

## 41. Embedded portal assets

For a small fixed Identity portal, static files may be compiled into the Go binary:

```go
package static

import _ "embed"

//go:embed indexEN.html
var IndexENHTML []byte

//go:embed portal.css
var PortalCSS []byte

//go:embed portal.js
var PortalJS []byte
```

Serve each with the correct content type.

For a much larger asset set, an `embed.FS` may be cleaner.

Embedding avoids runtime filesystem dependencies.

---

## 42. Browser security headers

Identity HTML/auth surfaces should maintain strict headers appropriate to authentication pages, including:

```text
X-Content-Type-Options: nosniff
Referrer-Policy: no-referrer
frame protection
no-store on sensitive responses
CSP appropriate to the portal
```

Do not weaken origin/CSP rules just to make local testing convenient; configure a legitimate local origin instead.

---

## 43. Adding an Identity feature

Classify the feature first:

```text
authentication
OIDC
application registry
assignment/authorization
delegation/introspection
recovery
operations
```

Implement through the existing layers:

```text
migration if persistent
domain/service logic
store methods
HTTP/CLI boundary
audit
config
tests
operational docs
```

Avoid placing SQL/business policy directly into HTTP handlers when a service/store boundary exists.

---

## 44. Adding an admin mutation

Checklist:

- Which Identity permission authorizes it?
- Is the actor enabled?
- Does it require recent WebAuthn?
- Is permission evaluated before step-up signaling?
- Is browser CSRF enforced at BFF?
- Is the bearer audience `mw-identity-admin`?
- Is the mutation audited?
- Is sensitive payload excluded from audit?
- Should related sessions/tokens be revoked?
- Are denial paths tested?

---

## 45. Adding an authentication method

A new method is not only UI.

It requires decisions for:

```text
credential persistence
proof/ceremony
auth_method
auth_time
assurance
OIDC propagation
delegated-token authentication context
step-up compatibility
recovery
revocation
audit
E2E
```

Decide explicitly whether it satisfies:

```text
standard
strong
urn:mw:webauthn
```

Only WebAuthn satisfies `urn:mw:webauthn`.

---

## 46. Testing baseline

Static:

```powershell
go -C .\mw-identity test ./...
go -C .\mw-identity vet ./...
docker compose config
```

OIDC:

```text
discovery
JWKS
signing
rotation overlap
client auth
PKCE
redirect exactness
code single-use
token expiry
userinfo
assignments
disabled app/user
```

WebAuthn:

```text
wrong RP
wrong origin
wrong challenge
expired challenge
replay
credential mismatch
user verification
registration
login
removal
last-passkey protection
```

TOTP:

```text
correct code
wrong code
±1 window
replay
secret encryption
enable/disable
```

Delegation:

```text
caller authentication
subject token active
audience allowed
scope narrowing
Identity-only scope rejection
expiry
introspection audience
wrong audience
revocation
```

Recovery:

```text
capability one-time
TTL
recovery_pending
session/token revocation
cancel
no email route
replacement passkey
old trust invalidation
audit redaction
```

Actual WebAuthn changes require a real browser ceremony checkpoint.

---

## 47. Code-review checklist

Before merging Identity work verify:

- Stable `sub` remains authoritative.
- Email is not used as permanent identity.
- Ordinary applications still require assignments.
- Superadmin exception remains narrow.
- Client secrets are stored only as hashes.
- Token/capability values are hashed where possible.
- Sensitive persistent material uses the correct keyring.
- Redirect URIs remain exact.
- PKCE remains mandatory.
- WebAuthn RP/origin remains strict.
- TOTP replay prevention remains intact.
- `auth_time` remains correct.
- TOTP cannot satisfy WebAuthn ACR.
- Token exchange remains audience/scope narrowed.
- Resource server cannot introspect another audience.
- Revocation fails closed.
- Recovery secrets are absent from logs.
- Audit is safe.
- Production configuration fails closed.

---

## 48. Non-goals

MW Identity is not:

- a generalized application database;
- a service configuration store;
- a Mail sender;
- an OAuth provider-token vault;
- a replacement for per-application domain authorization;
- an email-equality auto-linker;
- a generic proxy;
- a home for downstream project API keys.

Its durable role is:

> Provide stable human identity, strong authentication, registered application trust, assignment-based authorization, secure token issuance/delegation, introspection, revocation, and operator-controlled recovery.
