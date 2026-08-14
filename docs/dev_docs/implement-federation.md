# Implement Federation — MW OAuth and MW Email

## Purpose

This document defines the work required in **MW OAuth Service** and **MW Email Service** so they can participate in the MattediWorks control-plane federation built by `mw-plane-control`.

The control-plane team owns:

```text
MW Identity
MW BFF
OIDC login
human Identity subjects
application assignments
delegated downstream tokens
token introspection
BFF-side service orchestration
```

The service teams own:

```text
MW OAuth Service
MW Email Service
their admin APIs
their domain/business logic
their local/federated admin-auth boundary
resource-server token validation
service-specific authorization
service audit attribution
```

The objective is **not** to move OAuth or Mail business logic into the BFF.

The objective is:

```text
human administrator
        ↓
      MW BFF
        ↓
short-lived delegated token
        ↓
MW OAuth / MW Email
        ↓
existing service admin operations
```

---

# 1. Current Control-Plane Contract

Step 09 of `mw-plane-control` is complete and has passed E2E.

The control plane can now:

1. authenticate a human through MW Identity;
2. maintain the human session in MW BFF;
3. exchange the BFF's Identity token for a short-lived delegated token;
4. mint the delegated token for one exact downstream audience;
5. narrow delegated scopes;
6. preserve the human Identity subject;
7. preserve authentication context such as `auth_time` and `amr`;
8. introspect the delegated token from a trusted resource server.

Conceptual flow:

```text
human
  ↓
MW Identity
  ↓
MW BFF session
  ↓
OAuth token exchange
  ↓
opaque delegated token
  ↓
MW OAuth or MW Email
  ↓
authenticated introspection against MW Identity
```

The delegated token is intentionally **opaque**.

Downstream services MUST NOT attempt to parse or locally validate it as a JWT.

They validate it through MW Identity introspection.

---

# 2. Architecture Boundary

## 2.1 BFF responsibilities

MW BFF is responsible for:

```text
browser-facing admin session
CSRF at the browser/BFF boundary
requesting delegated tokens
caching delegated tokens safely
selecting the correct audience
selecting the minimum required scopes
calling downstream services
normalizing downstream failures where appropriate
```

## 2.2 Service responsibilities

MW OAuth and MW Email are responsible for:

```text
validating delegated Bearer tokens
checking active=true
checking exact audience
checking required scope
recovering the human Identity subject
performing business authorization
performing the actual domain operation
auditing the human actor
```

## 2.3 Business logic stays in the service

Examples:

```text
MW OAuth
    project creation
    API-key rotation
    callback-secret rotation
    OAuth provider policy
    rate limits
    audit
    metrics

MW Email
    project creation
    project key rotation
    email inspection
    provider/delivery state
    usage
    email audit/history
```

The BFF MUST NOT duplicate these rules.

---

# 3. Authentication Modes

Both services should support:

```env
ADMIN_AUTH_MODE=local
ADMIN_AUTH_MODE=hybrid
ADMIN_AUTH_MODE=federated
```

Recommended semantics:

## `local`

Existing service-local admin authentication only.

Used as the compatibility mode before federation.

## `hybrid`

Both are allowed:

```text
existing local admin session
OR
federated delegated Bearer token
```

This is the required migration mode.

Use this during rollout and E2E validation.

## `federated`

Administrative access is accepted only through the federated Identity/BFF path.

Local admin login/session endpoints should be disabled or return a deliberate non-success response.

Do not silently fall back to local authentication.

---

# 4. Resource-Server Registration

Each service must be represented in MW Identity as its own confidential resource server.

Do not share credentials between OAuth and Mail.

## MW OAuth

Conceptual registration:

```text
client/application:
    mw-oauth-resource

audience:
    mw-oauth

type:
    confidential
```

## MW Email

Conceptual registration:

```text
client/application:
    mw-mail-resource

audience:
    mw-mail

type:
    confidential
```

Each service receives its own introspection credentials.

These credentials are infrastructure secrets and MUST NOT be exposed to a browser.

---

# 5. Required Environment Configuration

Use service-specific names if preferred, but the semantic configuration must exist.

## MW OAuth

```env
ADMIN_AUTH_MODE=hybrid

MW_IDENTITY_INTERNAL_BASE_URL=http://<identity-private-address>:7100
MW_IDENTITY_INTROSPECTION_CLIENT_ID=<mw-oauth-resource-client-id>
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=<secret>
MW_IDENTITY_AUDIENCE=mw-oauth
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5
```

## MW Email

```env
ADMIN_AUTH_MODE=hybrid

MW_IDENTITY_INTERNAL_BASE_URL=http://<identity-private-address>:7100
MW_IDENTITY_INTROSPECTION_CLIENT_ID=<mw-mail-resource-client-id>
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=<secret>
MW_IDENTITY_AUDIENCE=mw-mail
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5
```

The public Identity issuer and the private network address are separate concepts.

For example:

```text
logical issuer:
https://identity.mattediworks.com

private service-to-service transport:
http://mw-identity:7100
```

Production topology may differ from local Docker topology.

Do not hardcode Docker hostnames into domain logic.

---

# 6. Introspection Contract

The downstream service receives:

```http
Authorization: Bearer <opaque-delegated-token>
```

It must introspect that token with MW Identity.

Conceptually:

```http
POST /oauth/introspect
Authorization: Basic <resource-server-client-credentials>
Content-Type: application/x-www-form-urlencoded

token=<opaque-token>
```

The active response provides the security context required by the service.

Expected concepts include:

```text
active
sub
aud
scope
auth_time
amr
```

The service MUST reject the request unless all relevant checks pass.

At minimum:

```text
active == true
sub is non-empty
aud contains the exact service audience
required scope is present
```

For MW OAuth:

```text
aud == mw-oauth
```

For MW Email:

```text
aud == mw-mail
```

Never authorize based only on `sub`.

Never authorize based only on successful introspection.

Audience and scope are mandatory authorization inputs.

---

# 7. Fail-Closed Requirements

Federated authentication MUST fail closed.

Reject the downstream request if:

```text
Authorization header is malformed
Bearer token is missing
Identity is unavailable
introspection times out
introspection returns non-2xx
response cannot be decoded
active=false
sub is empty
audience is wrong
scope is missing
required authentication context is invalid
```

Do not fall back from a failed federated token to local authentication for the same request.

Example of what MUST NOT happen:

```text
Bearer token present but invalid
        ↓
federated auth fails
        ↓
try local cookie instead
```

A presented Bearer token should select the federated authentication path explicitly.

---

# 8. Shared Federation Package

Both Go services should implement the same conceptual package.

Suggested structure:

```text
internal/federation/
├── client.go
├── middleware.go
├── principal.go
├── scopes.go
└── errors.go
```

Possible principal:

```go
type Principal struct {
    Subject  string
    Audience string
    Scopes   []string
    AuthTime time.Time
    AMR      []string
}
```

The exact structure may differ.

The important invariant is that downstream handlers can reliably obtain:

```text
stable human Identity subject
authorized scopes
authentication context
```

without knowing token/introspection implementation details.

---

# 9. Unified Admin Principal

Existing service business handlers should not become coupled directly to Identity.

Prefer an abstraction such as:

```go
type AdminPrincipal struct {
    Source string

    LocalAdminID *string

    IdentitySubject *string
    Scopes          []string
    AuthTime        *time.Time
    AMR             []string
}
```

or equivalent.

Conceptually:

```text
AdminPrincipal
    ├── local
    └── federated
```

This allows the service to run in `hybrid` mode without duplicating every handler.

Business/domain functions should receive a trusted principal after authentication middleware has completed.

---

# 10. Browser Security vs Federated Security

The current services have browser-oriented protections such as:

```text
Origin allowlists
local session cookies
CSRF
admin route concealment
```

These should remain for local browser authentication.

They are NOT the authentication mechanism for BFF-to-service requests.

Federated path:

```text
BFF
  ↓
Authorization: Bearer <delegated-token>
  ↓
service
  ↓
Identity introspection
```

Therefore the BFF should not need to fake a browser `Origin`.

Do not require a browser CSRF token for a server-to-server Bearer request.

---

# 11. Route Authentication Strategy

Recommended conceptual middleware:

```text
/admin/api/*
        │
        ├── Bearer token present
        │      ↓
        │   federated authentication
        │      ↓
        │   scope authorization
        │
        └── no Bearer token
               ↓
           local auth allowed?
               ↓
           session + Origin + CSRF as applicable
```

In `ADMIN_AUTH_MODE=federated`:

```text
no valid Bearer token
    ↓
deny
```

In `ADMIN_AUTH_MODE=local`:

```text
federated path disabled
```

In `ADMIN_AUTH_MODE=hybrid`:

```text
both paths available
```

---

# 12. MW OAuth — Required Federation Work

MW OAuth should be the **reference implementation**.

Once this works correctly, reuse the same federation pattern in MW Email.

Existing OAuth business/admin APIs should remain authoritative.

Do not create parallel BFF-side copies of them.

## 12.1 Suggested OAuth scopes

Recommended initial scopes:

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

Avoid one broad scope such as:

```text
oauth.admin
```

unless temporarily required during migration.

The BFF should request the minimum scopes required by the specific operation.

## 12.2 Suggested route mapping

### Projects

```text
GET /admin/api/projects
    oauth.projects.read

POST /admin/api/projects
    oauth.projects.write

PATCH /admin/api/projects/{id}
    oauth.projects.write
```

### Credentials

```text
POST /admin/api/projects/{id}/rotate-key
    oauth.credentials.rotate

POST /admin/api/projects/{id}/rotate-callback-secret
    oauth.credentials.rotate
```

### Google/OAuth policy

```text
GET /admin/api/projects/{id}/oauth/google
    oauth.policy.read

PUT /admin/api/projects/{id}/oauth/google
    oauth.policy.write
```

### Operations

```text
GET /admin/api/audit
GET /admin/api/audit/{id}
    oauth.audit.read

GET /admin/api/metrics
    oauth.metrics.read

GET /admin/api/server
GET /admin/api/docs
GET /admin/api/docs/{version}
    oauth.server.read
```

The existing route names remain the source of truth if they differ slightly.

Do not rename public APIs merely for federation.

---

# 13. MW OAuth — What Must Remain Unchanged

Federation applies to **human administrative access**.

It does NOT replace existing project/client authentication.

This flow must remain:

```text
consumer project backend
        ↓
Authorization: Bearer mw_live_...
        ↓
MW OAuth public API
```

The existing project API key remains the machine-to-machine credential for project OAuth operations.

The existing callback signing secret remains the independent MW OAuth → Project authentication mechanism.

Do not replace either with Identity delegated tokens.

The project-facing OAuth transaction architecture remains unchanged:

```text
project backend
→ MW OAuth
→ Google
→ MW OAuth callback
→ signed project callback
```

---

# 14. MW Email — Required Federation Work

MW Email should adopt the federation layer after the MW OAuth implementation has been proven.

Suggested initial scopes:

```text
mail.projects.read
mail.projects.write

mail.credentials.rotate

mail.emails.read
mail.usage.read
```

Possible future scopes:

```text
mail.providers.read
mail.providers.write
mail.audit.read
```

Only introduce scopes for real supported operations.

## 14.1 Suggested route mapping

### Projects

```text
GET /admin/api/projects
    mail.projects.read

POST /admin/api/projects
    mail.projects.write

PATCH /admin/api/projects/{id}
    mail.projects.write
```

### Credentials

```text
POST /admin/api/projects/{id}/rotate-key
    mail.credentials.rotate
```

### Email inspection

```text
GET /admin/api/emails
GET /admin/api/emails/{id}
    mail.emails.read
```

Add usage/audit scopes only when the corresponding APIs are actually exposed.

---

# 15. MW Email — Origin Concealment Change

MW Email currently applies browser/admin-origin concealment to `/admin/api/*`.

That logic must be adjusted for federation.

The target model is:

```text
local browser admin request
    ├─ expected Host
    ├─ allowed Origin
    ├─ local admin session
    └─ CSRF for mutations

federated BFF request
    ├─ expected network/Host policy as applicable
    ├─ Bearer delegated token
    ├─ Identity introspection
    ├─ aud=mw-mail
    └─ required mail.* scope
```

Do not require:

```http
Origin: <fake-admin-origin>
```

from MW BFF.

Origin is a browser security concept.

The delegated Bearer token and service-side authorization are the security boundary for federated requests.

Concealment may remain as defense in depth, but it MUST NOT become a substitute for authentication.

---

# 16. MW Email — What Must Remain Unchanged

Normal email-sending applications continue to authenticate using the existing project API key.

Existing stable client contract:

```http
POST /v1/emails
Authorization: Bearer mw_live_...
Idempotency-Key: ...
Content-Type: application/json
```

Do not migrate normal email senders to Identity delegated tokens.

Expected architecture after federation:

```text
application backend
    │
    │ mw_live_...
    ▼
MW Email /v1/emails
```

while administrative access becomes:

```text
human administrator
    ↓
MW BFF
    ↓
delegated token aud=mw-mail
    ↓
MW Email /admin/api/*
```

These are two different trust boundaries.

---

# 17. Audit Requirements

Federated actions must be attributable to the human Identity subject.

Do not audit only:

```text
mw-bff
```

as the actor.

The important actor is:

```text
sub = usr_...
```

Recommended safe audit context:

```text
actor_type
actor_identity_sub
actor_auth_time
actor_amr
delegated_audience
authorized_scope
request_id
```

Example:

```text
actor_type=federated
actor_identity_sub=usr_ABC123
delegated_audience=mw-oauth
authorized_scope=oauth.credentials.rotate
```

Never audit:

```text
raw delegated token
Identity introspection secret
local session token
CSRF token
project API key
provider access token
provider refresh token
```

If email/display-name information is required for UI convenience, treat it as mutable display metadata.

The stable actor key is the Identity `sub`.

---

# 18. Authentication Context / Step-Up Readiness

The federation middleware should preserve:

```text
auth_time
amr
```

even if initial authorization does not use them heavily.

This prepares sensitive operations for future policies such as:

```text
credential rotation requires recent authentication

or

credential rotation requires WebAuthn in amr
```

Do not throw away this information after introspection.

The service should not implement new MFA logic itself.

MW Identity remains responsible for authentication.

The downstream service only enforces requirements expressed in trusted authentication context.

---

# 19. HTTP Client Requirements

Identity introspection clients must use:

```text
bounded timeout
bounded response body
no automatic cross-host redirects
proper Basic authentication
connection reuse
context cancellation
```

Recommended default timeout:

```text
5 seconds
```

The exact value should remain configurable.

Never log:

```text
Authorization header
raw delegated token
introspection client secret
```

---

# 20. Introspection Cache

Initial implementation may introspect every delegated request.

A very small positive-result cache may be considered later, but it is NOT required for the first federation release.

If caching is introduced:

```text
cache lifetime <= delegated-token remaining lifetime
cache key = cryptographic hash of raw token
never store raw token as cache key
inactive tokens should not receive long negative caching
```

Correctness is more important than reducing the introspection call count.

---

# 21. Error Contract

Downstream service errors should distinguish authentication from authorization.

Recommended semantics:

```text
401
    missing Bearer credentials
    invalid token
    inactive token
    failed authentication

403
    valid federated principal
    wrong audience
    missing required scope
    business authorization denied
```

Internal Identity/network errors should fail closed.

Do not expose sensitive introspection diagnostics to the browser.

Structured server logs may contain safe reason codes.

Example safe reasons:

```text
federation.token_missing
federation.token_inactive
federation.introspection_failed
federation.audience_mismatch
federation.scope_missing
```

---

# 22. Testing Requirements — Shared

Each service must have unit tests for the federation package.

At minimum:

```text
valid active token
inactive token
wrong audience
empty subject
required scope present
required scope missing
multiple scopes
Identity 401/403/500
Identity timeout
invalid JSON
oversized response
Bearer header malformed
```

Middleware tests must prove:

```text
local mode accepts local auth and rejects federated auth
hybrid mode accepts local auth
hybrid mode accepts federated auth
federated mode rejects local-only access
invalid Bearer token never falls back to local session
```

---

# 23. Testing Requirements — MW OAuth

Minimum E2E federation proof:

```text
human logs into control plane
    ↓
BFF obtains delegated token aud=mw-oauth
    ↓
GET OAuth projects succeeds
    ↓
create/update test project succeeds
    ↓
human sub appears in OAuth audit
```

Security cases:

```text
delegated token aud=mw-mail
    → rejected by MW OAuth

token without oauth.projects.read
    → GET projects rejected

token with read but not write
    → GET succeeds
    → POST/PATCH rejected

inactive token
    → rejected
```

Credential rotation test:

```text
oauth.credentials.rotate present
    → rotation succeeds

scope absent
    → rotation rejected
```

---

# 24. Testing Requirements — MW Email

Minimum E2E federation proof:

```text
human logs into control plane
    ↓
BFF obtains delegated token aud=mw-mail
    ↓
GET Mail projects succeeds
    ↓
GET email history succeeds
    ↓
human sub appears in Mail audit/log context
```

Security cases:

```text
delegated token aud=mw-oauth
    → rejected by MW Email

token without mail.projects.read
    → project listing rejected

token without mail.emails.read
    → email listing rejected
```

Also prove that existing project mail delivery still works:

```text
POST /v1/emails
Authorization: Bearer mw_live_...
Idempotency-Key: ...
    → unchanged behavior
```

---

# 25. Rollout Strategy

Use this order.

## Phase A — MW OAuth hybrid mode

```text
local auth still available
federated auth added
federated E2E proven
audit attribution proven
```

Do not remove local auth yet.

## Phase B — BFF OAuth integration

Control-plane team adds real BFF OAuth routes and uses delegated:

```text
aud=mw-oauth
```

tokens.

## Phase C — MW OAuth federated mode

After production/staging confidence:

```env
ADMIN_AUTH_MODE=federated
```

Local admin login may then be disabled.

Keep an operator recovery path outside normal HTTP login if required.

## Phase D — MW Email hybrid mode

Reuse the proven OAuth federation pattern.

## Phase E — BFF Mail integration

Control-plane team adds delegated:

```text
aud=mw-mail
```

calls.

## Phase F — MW Email federated mode

Disable local human admin login only after federation has been proven.

---

# 26. Service-Team Deliverables

## MW OAuth team

Deliver:

```text
[ ] federation/introspection client
[ ] federated principal
[ ] local|hybrid|federated auth mode
[ ] audience validation for mw-oauth
[ ] oauth.* scope authorization
[ ] existing admin handlers wired to federated principal
[ ] federated human audit attribution
[ ] local auth preserved in hybrid mode
[ ] public project OAuth API unchanged
[ ] unit tests
[ ] integration tests
[ ] federation documentation
```

## MW Email team

Deliver:

```text
[ ] federation/introspection client
[ ] federated principal
[ ] local|hybrid|federated auth mode
[ ] audience validation for mw-mail
[ ] mail.* scope authorization
[ ] admin Origin gate adjusted for server-to-server federation
[ ] existing admin handlers wired to federated principal
[ ] federated human audit attribution
[ ] local auth preserved in hybrid mode
[ ] /v1/emails project-key auth unchanged
[ ] unit tests
[ ] integration tests
[ ] federation documentation
```

---

# 27. Control-Plane Team Deliverables

These are NOT part of the OAuth/Mail service implementation.

The control-plane team will separately implement:

```text
MW OAuth resource registration
MW Email resource registration

BFF OAuth service client
BFF Mail service client

BFF route → audience/scope mapping

delegated-token acquisition

delegated-token cache use

admin-facing unified API facade
```

The two teams should agree on:

```text
audience names
scope names
service base URLs
resource-server client IDs
error contract
request IDs / trace correlation
```

before final E2E.

---

# 28. Explicit Non-Goals

This federation work does NOT:

```text
move OAuth business logic into MW BFF
move Mail business logic into MW BFF

replace project API keys
replace Mail send API keys
replace OAuth callback HMAC secrets

turn delegated tokens into JWTs
allow services to mint Identity tokens
give services user passwords
give services Identity private signing keys

make MW OAuth an Identity provider
make MW Email an Identity provider

require browser Origin for BFF-to-service calls

introduce service-specific MFA
introduce service-specific passkeys
```

Authentication remains centralized in MW Identity.

---

# 29. Security Invariants

The implementation is complete only if these remain true:

```text
1. Human identity key is the immutable Identity subject (`sub`).

2. Delegated tokens are accepted only by their intended audience.

3. Scope is checked at the downstream service.

4. BFF cannot convert one service token into authority over another service.

5. OAuth and Mail never receive the user's Identity password.

6. OAuth and Mail never receive Identity signing private keys.

7. Raw delegated tokens are never persisted in service audit/log storage.

8. Existing application/project credentials remain independent from human federation.

9. A failed introspection fails closed.

10. A failed federated request cannot fall back to a local cookie.

11. Business logic stays in the owning service.

12. Sensitive actions are attributable to the human `sub`, not merely to MW BFF.
```

---

# 30. Completion Gates

## Step 10 / MW OAuth federation is ready for control-plane integration when

```text
MW OAuth starts in hybrid mode

valid delegated token:
    aud=mw-oauth
    required oauth.* scope
    stable human sub
        ↓
existing admin operation succeeds

wrong audience
    ↓
rejected

missing scope
    ↓
rejected

Identity unavailable
    ↓
fails closed

local admin login
    ↓
still works in hybrid mode

existing project API-key OAuth flow
    ↓
unchanged

audit
    ↓
records human Identity sub
```

At that point the OAuth team should hand the control-plane team:

```text
service base URL
resource audience
introspection client registration requirements
final scope list
route-to-scope mapping
test credentials/setup procedure
```

## Step 11 / MW Email federation is ready when

The same invariants pass for:

```text
aud=mw-mail
mail.* scopes
```

and normal:

```text
POST /v1/emails
Authorization: Bearer mw_live_...
```

continues working unchanged.

---

# 31. Recommended Implementation Order

Use MW OAuth as the reference implementation:

```text
1. define shared federation contract
2. implement MW OAuth introspection client
3. implement MW OAuth principal/middleware
4. add OAuth scope mapping
5. add hybrid mode
6. add OAuth federation tests
7. prove BFF → OAuth E2E
8. extract/reuse federation pattern
9. implement MW Email equivalent
10. adjust Mail Origin/concealment boundary
11. prove BFF → Mail E2E
12. later switch each service to federated mode
```

Do not implement OAuth and Mail independently with two incompatible federation models.

They should use the same security semantics even if their internal code is not shared as a Go module.

---

# 32. Final Target

The final administrative architecture should be:

```text
                     ┌────────────────────┐
                     │     MW Identity    │
                     │                    │
                     │ users              │
                     │ assignments        │
                     │ authentication     │
                     │ token exchange     │
                     │ introspection      │
                     └─────────┬──────────┘
                               │
                               │
                     ┌─────────▼──────────┐
                     │       MW BFF       │
                     │                    │
                     │ browser session    │
                     │ CSRF               │
                     │ admin facade       │
                     │ token delegation   │
                     └──────┬───────┬─────┘
                            │       │
             aud=mw-oauth   │       │   aud=mw-mail
                            │       │
                  ┌─────────▼──┐ ┌──▼──────────┐
                  │  MW OAuth  │ │  MW Email   │
                  │            │ │             │
                  │ OAuth      │ │ email       │
                  │ projects   │ │ projects    │
                  │ policy     │ │ delivery    │
                  │ audit      │ │ history     │
                  └────────────┘ └─────────────┘
```

The control plane authenticates and delegates.

Each service authorizes and executes its own business operations.
