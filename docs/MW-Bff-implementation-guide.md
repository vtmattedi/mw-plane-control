# MW BFF — Implementation Guide

**Component:** `mw-bff`  
**Role:** browser-facing security boundary and strict administrative facade  
**Version baseline:** MW Control Plane v1.0

---

## 1. Mission

MW BFF keeps browser trust separate from internal service trust.

Its durable responsibility is:

> Authenticate the browser session, enforce browser-side security and assurance, acquire narrowly scoped delegated authority, and expose only explicitly approved administrative operations.

It is **not** a business-logic service and must never become a generic internal proxy.

---

## 2. Core responsibilities

MW BFF owns:

- OIDC relying-party flow against MW Identity;
- OIDC state, nonce and PKCE transaction state;
- browser session creation and validation;
- browser CSRF;
- safe `return_to`;
- Identity ID-token validation;
- encrypted Identity admin/subject bearer persistence required for the control plane;
- Identity Admin facade;
- delegated-token acquisition;
- encrypted delegated-token cache;
- explicit downstream route mapping;
- route-to-scope mapping;
- recent-WebAuthn enforcement;
- bounded internal HTTP clients;
- safe downstream response forwarding;
- request-ID propagation.

It does not own:

- Identity password/WebAuthn/TOTP verification;
- application assignments;
- OAuth business logic;
- Mail business logic;
- downstream project/user models.

---

## 3. Public path model

The browser uses:

```text
/api/*
```

The gateway removes `/api` before forwarding to BFF.

Examples:

```text
Browser                 BFF
/api/auth/me       ->   /auth/me
/api/identity/...  ->   /identity/...
/api/oauth/...     ->   /oauth/...
/api/mail/...      ->   /mail/...
```

Do not register the public `/api` prefix inside BFF handlers.

---

## 4. Browser authentication API

External routes:

```text
GET  /api/auth/login
GET  /api/auth/callback
GET  /api/auth/me
POST /api/auth/csrf
POST /api/auth/logout
GET  /api/auth/step-up
```

Internal BFF routes:

```text
GET  /auth/login
GET  /auth/callback
GET  /auth/me
POST /auth/csrf
POST /auth/logout
GET  /auth/step-up
```

---

## 5. OIDC login flow

```text
GET /auth/login
    ↓
validate return_to
    ↓
generate state + nonce + PKCE
    ↓
persist one-time transaction
    ↓
redirect to Identity /oauth/authorize
    ↓
callback with code + state
    ↓
claim transaction atomically
    ↓
server-to-server code exchange
    ↓
validate ID token
    ↓
create BFF session
    ↓
redirect safe return_to
```

OIDC transaction storage should contain only the minimum safe state.

Do not persist raw state/nonce where hashes are sufficient.

The PKCE verifier is sensitive and must be protected using the BFF versioned encryption keyring.

---

## 6. ID-token validation

Fail closed unless all required checks pass:

```text
compact JWS shape
alg = RS256
kid present in Identity JWKS
valid RSA signature
iss exact
aud contains BFF client_id
exp valid
iat valid
auth_time valid
nonce matches
sub present
```

Separate canonical protocol identity from transport:

```env
BFF_IDENTITY_ISSUER=https://identity.mattediworks.com
BFF_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
```

Use the public issuer for OIDC validation.

Use the internal base URL only for server-to-server transport.

---

## 7. BFF session model

The browser receives an opaque session cookie.

Properties:

```text
HttpOnly
SameSite=Strict
Path=/
host-only
Secure in production
```

The raw session token is not stored in SQLite; store its hash.

Trusted server-side session state includes values such as:

```text
Identity issuer
Identity sub
email
display name
auth_time
auth_method
expiry
CSRF hash
encrypted Identity admin/subject token
```

Never expose the Identity bearer to JavaScript.

---

## 8. CSRF

Browser mutations require:

```text
valid BFF session
+
valid current CSRF token
```

Frontend obtains CSRF via:

```text
POST /api/auth/csrf
```

The raw CSRF value is presented in the expected header and compared with server-side trusted state/hash.

Do not forward browser CSRF to downstream services.

Do not let a delegated bearer operation fall back to browser-cookie semantics.

---

## 9. Safe `return_to`

`return_to` must be a local application path/query.

Reject:

```text
https://attacker.example/
//attacker.example/
\attacker.example
other absolute/network-path URLs
```

Login and step-up flows must not introduce open redirects.

---

## 10. Identity Admin facade

Identity admin resource is:

```text
Identity /admin/api/*
```

Browser sees:

```text
/api/identity/*
```

BFF internally handles:

```text
/identity/*
```

Identity facade requirements:

- valid BFF session;
- BFF CSRF for browser mutations;
- Identity admin bearer retrieved/decrypted server-side;
- bounded request bodies;
- bounded response reads;
- bounded timeout;
- no redirect following;
- safe header allowlist;
- request-ID propagation;
- no bearer exposure to browser.

BFF's Identity application requires:

```text
scope = identity.admin
aud   = mw-identity-admin
```

---

## 11. Delegated downstream flow

Never forward the Identity-admin bearer directly to an arbitrary service.

Use token exchange:

```text
BFF session
    ↓
decrypt Identity subject token in memory
    ↓
Identity token exchange
    ↓
aud=<service>
scope=<narrow service scope>
    ↓
opaque short-lived delegated token
    ↓
service admin API
```

The delegated-token cache is a performance optimization only. It must remain bounded by token expiry and exact audience/scope semantics.

If token exchange fails, fail the request.

---

## 12. Token-exchange request

Conceptual request:

```text
grant_type=urn:ietf:params:oauth:grant-type:token-exchange
subject_token=<Identity token>
subject_token_type=urn:ietf:params:oauth:token-type:access_token
requested_token_type=urn:ietf:params:oauth:token-type:access_token
audience=<exact resource audience>
scope=<required service scopes>
```

Do not delegate Identity-only scopes:

```text
openid
profile
email
identity.admin
```

---

## 13. Strict facade model

Every route must be explicit.

Conceptually:

```go
type Route struct {
    Method          string
    PublicPattern   string
    DownstreamPath  string
    Scope           string
    RequireStepUp   bool
}
```

The implementation may use different types, but the same security contract must remain.

Never add:

```text
/api/mail/proxy/*
/api/oauth/proxy/*
/api/service/{arbitrary-path}
```

Unknown method/path combinations must return an error/404 rather than being forwarded.

---

## 14. MW OAuth facade

Audience:

```text
mw-oauth
```

Scopes:

```text
oauth.projects.read
oauth.projects.write
oauth.credentials.rotate
oauth.policy.read
oauth.policy.write
oauth.audit.read
oauth.metrics.read
oauth.server.read
```

Representative internal BFF routes:

```text
GET    /oauth/projects
POST   /oauth/projects
PATCH  /oauth/projects/{id}

POST   /oauth/projects/{id}/rotate-key
POST   /oauth/projects/{id}/rotate-callback-secret

GET    /oauth/projects/{id}/oauth/google
PUT    /oauth/projects/{id}/oauth/google

GET    /oauth/audit
GET    /oauth/audit/{id}
GET    /oauth/metrics
GET    /oauth/server
GET    /oauth/docs
GET    /oauth/docs/{version}
```

Credential rotation and sensitive provider-policy changes require recent WebAuthn.

---

## 15. MW Mail facade

Audience:

```text
mw-mail
```

Scopes:

```text
mail.projects.read
mail.projects.write
mail.credentials.rotate
mail.emails.read
mail.usage.read
```

Representative internal BFF routes:

```text
GET    /mail/health
GET    /mail/projects
POST   /mail/projects
PATCH  /mail/projects/{id}

GET    /mail/projects/{id}/request-domains
PUT    /mail/projects/{id}/request-domains

POST   /mail/projects/{id}/rotate-key

GET    /mail/emails
GET    /mail/emails/{id}
GET    /mail/emails/by-project
```

Mail project credential rotation requires recent WebAuthn.

---

## 16. Strong-auth session context

BFF retains trusted authentication context needed for assurance decisions:

```text
auth_time
auth_method
assurance
recent_webauthn
```

Typical interpretation:

```text
WebAuthn -> high assurance
TOTP     -> fallback assurance
password -> legacy/lower assurance
```

Default freshness:

```env
BFF_STRONG_AUTH_MAX_AGE_MINUTES=5
```

Compute freshness using server time and trusted Identity `auth_time`, never a browser timestamp.

---

## 17. Step-up flow

Sensitive route:

```text
session valid
    ↓
route/authorization valid
    ↓
recent WebAuthn?
    yes -> continue
    no  -> 403 step_up_required
```

Response:

```json
{
  "error": {
    "code": "step_up_required",
    "step_up_url": "/api/auth/step-up"
  }
}
```

BFF starts OIDC with:

```text
acr_values=urn:mw:webauthn
max_age=<freshness>
```

TOTP must not satisfy explicit WebAuthn step-up.

---

## 18. Downstream HTTP client requirements

Every downstream client must have:

```text
explicit base URL
optional logical Host
bounded timeout
redirects disabled
bounded response read
bounded request body
safe request-ID propagation
Authorization: Bearer <delegated token>
```

Do not forward:

```text
browser cookies
browser Origin
BFF CSRF
unrelated browser headers
```

Do not use `http.DefaultClient` blindly without reviewing timeout and redirect behavior.

---

## 19. Transport URL vs logical Host

Local/native services may require a production-style Host while using a local transport URL.

Example:

```env
BFF_MAIL_BASE_URL=http://host.docker.internal:8082
BFF_MAIL_HOST=mailgo.mattediworks.com

BFF_OAUTH_BASE_URL=http://host.docker.internal:8282
BFF_OAUTH_HOST=oauth.mattediworks.com
```

Transport location and logical HTTP Host are different concepts. Preserve that distinction.

---

## 20. Request and response limits

Apply body limits before reading untrusted browser input.

Use bounded response reads from downstream services.

A downstream service must not be able to cause unbounded BFF buffering.

Return safe error classes without leaking internal credentials or transport details.

---

## 21. Request IDs

Propagate normalized:

```text
X-Request-ID
```

Request IDs are for correlation, never authorization.

If BFF is invoked internally without the gateway, ensure a safe request ID is still generated/validated.

---

## 22. Logging

Safe BFF logs may contain:

```text
request_id
method
path
status
duration
Identity sub
downstream service
safe error class
```

Do not log:

```text
session cookie
CSRF
OIDC state
nonce
authorization code
PKCE verifier
Identity bearer
delegated bearer
client secret
raw sensitive body
```

Use structured logging.

---

## 23. Adding an endpoint to an existing service

Example service endpoint:

```text
POST /admin/api/projects/{id}/verify-domain
```

Service team defines:

```text
method/path
request/response schema
required scope
sensitivity
audit behavior
```

BFF implementation sequence:

1. Add explicit route match.
2. Add exact downstream path construction.
3. Add required delegated scope.
4. Decide whether fresh WebAuthn is required.
5. Keep session requirement.
6. Keep CSRF on browser mutation.
7. Keep request/response limits.
8. Reuse the service client.
9. Add success and denial tests.
10. Verify an unmapped adjacent route still returns 404.

The frontend continues to call only `/api/<service>/...`.

---

## 24. Adding a new scope

If a service introduces:

```text
mail.domains.verify
```

update as needed:

- BFF route mapping;
- BFF delegation scope configuration;
- BFF Identity application allowed scopes;
- service-side scope enforcement;
- integration tests.

Do not reuse an unrelated broad scope merely to avoid configuration work unless that scope genuinely represents the intended authorization boundary.

---

## 25. Adding a new service

For `mw-storage`:

### Configuration

Add:

```text
base URL
logical Host if required
audience = mw-storage
HTTP timeout
```

### Client

Create a bounded internal admin client.

### Facade

Expose only explicit `/storage/*` operations.

### Delegation

Request:

```text
aud = mw-storage
scope = storage.*
```

### Identity application

BFF's Identity application must allow the required service audience and scopes.

### Strong auth

Mark credential/security-changing actions as fresh-WebAuthn operations.

### Tests

Prove:

```text
valid route succeeds
missing session fails
mutation without CSRF fails
wrong/missing scope fails
downstream outage fails closed
step-up required where sensitive
unmapped route returns 404
browser never receives delegated token
```

---

## 26. Do not build a universal proxy

A reusable internal service/facade framework is acceptable.

Good abstraction:

```text
service config
+
explicit route declarations
+
scope declarations
+
step-up declarations
```

Bad abstraction:

```text
arbitrary browser path
    -> arbitrary downstream path
```

The explicit facade is part of the security boundary.

---

## 27. Local frontend development

Recommended:

```text
Browser http://localhost:5173
    -> Vite /api proxy
    -> Gateway :3050
    -> BFF
```

Browser code should use relative paths:

```text
/api/auth/me
/api/identity/...
/api/oauth/...
/api/mail/...
```

Do not point browser code directly to `localhost:7200`.

A dedicated local BFF OIDC application should use:

```text
redirect_uri=http://localhost:5173/api/auth/callback
BFF_PUBLIC_BASE_URL=http://localhost:5173
```

The Vite proxy preserves `/api`; the gateway owns prefix removal.

---

## 28. Configuration categories

Core BFF config includes:

```env
BFF_PUBLIC_BASE_URL=
BFF_IDENTITY_ISSUER=
BFF_IDENTITY_INTERNAL_BASE_URL=

BFF_OIDC_CLIENT_ID=
BFF_OIDC_CLIENT_SECRET=

BFF_TRANSACTION_KEYS=
BFF_TRANSACTION_ACTIVE_KEY_VERSION=1

BFF_OIDC_TRANSACTION_TTL_MINUTES=10
BFF_ADMIN_SESSION_TTL_HOURS=8
BFF_OIDC_CLOCK_SKEW_SECONDS=60
BFF_IDENTITY_HTTP_TIMEOUT_SECONDS=5

BFF_DELEGATION_SCOPES=
BFF_DELEGATION_CACHE_SKEW_SECONDS=30

BFF_STRONG_AUTH_MAX_AGE_MINUTES=5
```

Service-specific:

```env
BFF_OAUTH_BASE_URL=
BFF_OAUTH_HOST=
BFF_OAUTH_AUDIENCE=mw-oauth
BFF_OAUTH_HTTP_TIMEOUT_SECONDS=5

BFF_MAIL_BASE_URL=
BFF_MAIL_HOST=
BFF_MAIL_AUDIENCE=mw-mail
BFF_MAIL_HTTP_TIMEOUT_SECONDS=5
```

Secrets are runtime configuration, never frontend configuration.

---

## 29. Persistence

BFF owns its own SQLite database and migration history.

Required behavior:

```text
WAL
foreign_keys=ON
busy_timeout=5000
```

Do not:

- share Identity's database;
- query Identity tables directly;
- couple BFF migrations to Identity database internals.

Cross-component contracts go through HTTP/OIDC interfaces.

---

## 30. Testing requirements

Static gate:

```powershell
go -C .\mw-bff test ./...
go -C .\mw-bff vet ./...
docker compose config
```

Auth changes should test:

```text
state
nonce
PKCE
invalid callback
ID-token signature
issuer
audience
expiry
session creation
logout
CSRF
safe return_to
```

Facade changes should test:

```text
route matching
scope mapping
body limits
CSRF
session
delegation
wrong audience/scope
downstream error
step-up
unmapped 404
```

WebAuthn/step-up changes require a real browser checkpoint in addition to unit/API tests.

---

## 31. Code-review checklist

Before merging BFF work verify:

- Is business logic still in the owning service?
- Is the browser route explicit?
- Is the downstream route explicit?
- Is the delegated scope exact?
- Does mutation require CSRF?
- Does a sensitive action require fresh WebAuthn?
- Are request/response sizes bounded?
- Are HTTP timeouts bounded?
- Are redirects disabled?
- Are browser cookies/Origin/CSRF excluded downstream?
- Is request ID propagated safely?
- Are tokens absent from logs?
- Does an unknown adjacent route return 404?
- Does the BFF application have the required Identity scope/audience?
- Are both positive and denial paths tested?

---

## 32. Non-goals

MW BFF is not:

- a generic API gateway;
- an application database aggregator;
- a replacement for downstream authorization;
- a home for OAuth/Mail business logic;
- a browser bearer-token issuer;
- a substitute for resource-server introspection;
- an Identity password/WebAuthn/TOTP verifier.

Its durable role remains a thin, explicit, strongly authenticated administrative boundary.
