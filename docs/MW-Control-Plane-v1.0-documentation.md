# MW Control Plane v1.0 — Technical Documentation

**Status:** v1.0 baseline; v1.1 adds the MQTT module (section 30)  
**Scope:** MW Plane Control shared administrative control plane  
**Components:** MW Identity, MW BFF, Gateway, MW Web Admin, delegated MattediWorks services  
**Reference state:** Steps 01–13 implemented and tested through `STEP 13 API/OPS PASS`

---

## 1. Purpose

MW Control Plane is the shared administrative trust boundary for MattediWorks services. It centralizes human authentication, application administration, assignments, strong authentication, browser session handling, CSRF, downstream delegated authorization, audit attribution, and operational hardening.

It does **not** replace the business APIs or project credentials of the services it administers.

```text
Human administrator
    -> MW Web Admin
    -> Gateway
    -> MW BFF
    -> MW Identity / delegated service

Application/project client
    -> service public API
    -> service-specific project credential
```

These trust paths remain independent.

---

## 2. Canonical architecture

```text
                              ┌─────────────────────┐
                              │     MW Identity     │
                              │ users               │
                              │ applications        │
                              │ assignments         │
                              │ OIDC                │
                              │ WebAuthn / TOTP     │
                              │ token exchange      │
                              │ introspection       │
                              │ recovery            │
                              └──────────┬──────────┘
                                         │
                                         ▼
┌───────────────────┐          ┌─────────────────────┐
│   MW Web Admin    │─────────►│       MW BFF        │
│ browser only      │ /api/*   │ browser session     │
│ no bearer tokens  │          │ CSRF                │
└─────────┬─────────┘          │ step-up             │
          │                    │ strict facades      │
          ▼                    │ delegation cache    │
┌───────────────────┐          └──────┬───────┬──────┘
│     Gateway       │                 │       │
│ Host routing      │                 ▼       ▼
│ /api stripping    │           ┌──────────┐ ┌──────────┐
│ abuse controls    │           │ MW OAuth │ │ MW Mail  │
│ request IDs       │           │ business │ │ business │
│ safe logging      │           │ logic    │ │ logic    │
└───────────────────┘           └──────────┘ └──────────┘
```

Future admin-managed services follow the same pattern.

---

## 3. Ownership boundaries

### MW Identity owns

- stable human `sub`;
- local Identity users;
- global superadmin bootstrap;
- password authentication;
- Identity sessions;
- OIDC discovery and JWKS;
- OIDC Authorization Code + PKCE;
- application registry;
- application assignments;
- Identity-admin permissions;
- WebAuthn/passkeys;
- TOTP fallback;
- authentication assurance;
- token exchange;
- token introspection;
- signing keys;
- user/session/token revocation;
- operator recovery;
- Identity audit.

Identity does **not** own the local/domain user tables of consuming applications.

### MW BFF owns

- OIDC relying-party behavior for MW Web Admin;
- browser session;
- browser CSRF;
- safe `return_to`;
- OIDC state, nonce, and PKCE transaction state;
- encrypted Identity admin/subject token persistence needed by the control plane;
- delegated-token acquisition and cache;
- strict route-to-downstream mapping;
- strict route-to-scope mapping;
- recent-WebAuthn gates for sensitive actions;
- bounded internal HTTP clients.

BFF does not own downstream business logic.

### Gateway owns

- public Host routing;
- unknown-Host rejection;
- `/api/` prefix removal before BFF;
- forwarding-header normalization;
- request-ID normalization;
- path-only access logging;
- gateway-level abuse controls;
- public security headers;
- proxy timeouts.

### MW Web Admin owns

- UI and human workflow;
- `/api/auth/*`;
- `/api/identity/*`;
- `/api/oauth/*`;
- `/api/mail/*`;
- handling `step_up_required`.

The browser must never receive Identity or delegated bearer tokens.

### Downstream services own

Each service owns its business logic and admin API. For example, MW Mail owns mail projects, usage, email history and Mail credential rotation; MW OAuth owns OAuth projects, provider configuration, callback secrets and OAuth audit/metrics.

---

## 4. Stable identity

The stable human principal is:

```text
Identity issuer + sub
```

For first-party Identity:

```text
issuer = https://identity.mattediworks.com
sub    = usr_...
```

Email is profile data, not the immutable foreign key.

Application-local account links and Identity assignments remain separate:

```text
Identity assignment:
    May usr_123 use Application A?

Application-local mapping:
    Which local user does usr_123 represent?
```

A relying application that already has users owns its own `issuer + subject -> local_user_id` mapping.

---

## 5. Identity application registry

Applications describe OIDC clients and resource servers.

Important fields include:

```text
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
```

Application types:

```text
confidential
public
```

Authentication policies:

```text
standard
strong
```

`client_id` and application type are immutable after creation.

Lifecycle:

```text
enabled <-> disabled
enabled/disabled -> archived
```

Archived applications are terminal.

Confidential secrets are returned once and stored only as hashes. Rotation invalidates the previous secret immediately.

---

## 6. Assignments

Assignments bind:

```text
application_id + Identity user/sub + role
```

Roles:

```text
user
admin
superadmin
```

Ordinary OIDC authorization requires an explicit assignment.

The only bootstrap exception is:

```text
global Identity superadmin
+
application requests identity.admin
+
application audience includes mw-identity-admin
```

This exception is only for Identity administration and must not become a universal implicit assignment.

---

## 7. OIDC contract

Public Identity endpoints include:

```text
GET  /.well-known/openid-configuration
GET  /.well-known/jwks.json
GET  /oauth/authorize
POST /oauth/token
GET  /oauth/userinfo
POST /oauth/introspect
```

Core invariants:

- Authorization Code only;
- PKCE mandatory;
- only `S256`;
- exact redirect URI matching;
- required state under MW policy;
- one-time authorization codes;
- authorization codes stored only as hashes;
- confidential-client authentication;
- opaque access/delegated tokens;
- RS256 ID tokens;
- stable `sub`;
- disabled/archived apps fail closed;
- ordinary applications require assignments.

Baseline lifetimes:

```text
authorization code: 5 minutes
access token:       60 minutes
ID token:            5 minutes
delegated token:     5 minutes
```

---

## 8. Signing and JWKS

Identity uses RSA signing keys:

```text
alg = RS256
use = sig
unique kid
```

Private signing keys are encrypted before SQLite persistence with a versioned AES-256-GCM keyring.

```env
IDENTITY_SIGNING_KEYS=1:<base64url-32-byte-key>
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=1
```

Rotation:

```powershell
docker compose exec mw-identity /app/rotate-signing-key
```

Rotation creates a new active key and keeps the retired public JWK available so recently issued tokens remain verifiable.

Step 13 retention:

```env
IDENTITY_SIGNING_RETIRED_RETENTION_HOURS=72
```

Production validation must reject an unsafe retention window.

---

## 9. Browser authentication

Canonical flow:

```text
Browser
   -> /api/auth/login
BFF
   -> state + nonce + PKCE
   -> Identity /oauth/authorize
Identity
   -> strong authentication
   -> authorization code
Browser
   -> /api/auth/callback
BFF
   -> token exchange + ID-token validation
   -> opaque BFF session
```

Browser-facing auth routes:

```text
GET  /api/auth/login
GET  /api/auth/callback
GET  /api/auth/me
POST /api/auth/csrf
POST /api/auth/logout
GET  /api/auth/step-up
```

The gateway removes `/api`, so BFF internally handles `/auth/*`.

The browser session cookie is host-only, `HttpOnly`, `SameSite=Strict`, `Path=/`, and `Secure` outside local/development/test.

---

## 10. Strong authentication

Assurance hierarchy:

```text
Primary:   WebAuthn/passkey
Fallback:  TOTP
Emergency: manual operator recovery
```

Passwords remain available for compatible `standard` application flows and migration/recovery scenarios. Password-only assurance is not sufficient for a `strong` application's normal privileged authorization.

### WebAuthn

Production baseline:

```env
IDENTITY_WEBAUTHN_RP_ID=identity.mattediworks.com
IDENTITY_WEBAUTHN_RP_NAME=MW Identity
IDENTITY_WEBAUTHN_ORIGINS=https://identity.mattediworks.com
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS=1:<base64url-32-byte-key>
IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION=1
IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES=5
```

Policy:

```text
discoverable credential = required
user verification       = required
attestation              = none
```

### TOTP

```env
IDENTITY_TOTP_SECRET_KEYS=1:<base64url-32-byte-key>
IDENTITY_TOTP_SECRET_ACTIVE_KEY_VERSION=1
IDENTITY_TOTP_ISSUER=MW Identity
```

Parameters:

```text
HMAC:   SHA-1
digits: 6
period: 30 seconds
window: current ±1 step
```

TOTP secrets are encrypted at rest and accepted timesteps are replay-protected.

### Explicit WebAuthn step-up

Sensitive actions require fresh WebAuthn:

```text
GET /api/auth/step-up
```

BFF requests Identity with:

```text
acr_values=urn:mw:webauthn
max_age=<freshness window>
```

Default:

```env
BFF_STRONG_AUTH_MAX_AGE_MINUTES=5
```

A TOTP-backed session may receive:

```json
{
  "error": {
    "code": "step_up_required",
    "step_up_url": "/api/auth/step-up"
  }
}
```

TOTP does not satisfy an explicit WebAuthn ACR.

---

## 11. Operator recovery

There is intentionally no email recovery in v1.0.

Start:

```powershell
docker compose --env-file .env.production run --rm `
  --entrypoint /app/recover-identity `
  mw-identity `
  --email admin@example.com
```

Starting recovery:

- marks the user recovery-pending;
- revokes Identity sessions;
- revokes authorization codes;
- revokes access/delegated tokens;
- generates a short-lived one-time recovery capability;
- stores only its hash;
- returns the raw capability once.

Successful recovery replaces the passkey trust path, removes old TOTP fallback, invalidates the capability, and requires a fresh normal WebAuthn login before privileged work.

Cancel:

```powershell
docker compose --env-file .env.production run --rm `
  --entrypoint /app/recover-identity `
  mw-identity `
  --cancel `
  --email admin@example.com
```

Cancellation does not resurrect already revoked sessions.

---

## 12. Identity administration

Identity's admin resource is under:

```text
/admin/api/*
```

Representative routes:

```text
GET    /admin/api/whoami

GET    /admin/api/applications
POST   /admin/api/applications
GET    /admin/api/applications/{id}
PATCH  /admin/api/applications/{id}

POST   /admin/api/applications/{id}/disable
POST   /admin/api/applications/{id}/enable
POST   /admin/api/applications/{id}/archive
POST   /admin/api/applications/{id}/rotate-secret

GET    /admin/api/users
POST   /admin/api/users/{user_id}/revoke-sessions

GET    /admin/api/applications/{id}/assignments
PUT    /admin/api/applications/{id}/assignments/{user_id}
DELETE /admin/api/applications/{id}/assignments/{user_id}

GET    /admin/api/audit
```

The browser does not call these directly. BFF exposes them under `/api/identity/*`.

Mutations require:

```text
BFF session
+ BFF CSRF
+ Identity permission
+ recent WebAuthn where required
```

---

## 13. Delegated downstream tokens

BFF does not forward its long-lived Identity-admin bearer to downstream services.

Flow:

```text
human BFF session
    -> decrypt Identity subject token in memory
    -> Identity token exchange
    -> short-lived opaque delegated token
    -> downstream service
```

Conceptual token exchange:

```text
grant_type=urn:ietf:params:oauth:grant-type:token-exchange
subject_token=<Identity subject token>
subject_token_type=urn:ietf:params:oauth:token-type:access_token
requested_token_type=urn:ietf:params:oauth:token-type:access_token
audience=<exact downstream audience>
scope=<narrow service scopes>
```

Delegated tokens retain trusted human context such as `sub`, audience, scope, `auth_time`, authentication method and expiry.

Identity-only scopes such as `openid`, `profile`, `email` and `identity.admin` are not delegated as service permissions.

---

## 14. Introspection invariant

A resource server authenticates to:

```text
POST /oauth/introspect
```

The critical invariant is:

```text
token audience == introspecting application's client_id
```

Therefore:

```text
MW OAuth:
client_id = mw-oauth
aud       = mw-oauth

MW Mail:
client_id = mw-mail
aud       = mw-mail
```

Future service example:

```text
MW Storage:
client_id = mw-storage
aud       = mw-storage
```

Do not use mismatched resource registration names unless the introspection contract is intentionally redesigned.

---

## 15. MW OAuth facade

BFF exposes explicit routes such as:

```text
/api/oauth/projects
/api/oauth/projects/{id}
/api/oauth/projects/{id}/rotate-key
/api/oauth/projects/{id}/rotate-callback-secret
/api/oauth/projects/{id}/oauth/google
/api/oauth/audit
/api/oauth/audit/{id}
/api/oauth/metrics
/api/oauth/server
/api/oauth/docs
/api/oauth/docs/{version}
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

There is no generic OAuth proxy.

Service-level Google credentials belong to MW OAuth runtime/deployment configuration:

```env
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
```

They are not browser configuration and not ordinary per-project control-plane values.

---

## 16. MW Mail facade

Current browser routes include:

```text
GET   /api/mail/health
GET   /api/mail/projects
POST  /api/mail/projects
PATCH /api/mail/projects/{id}
GET   /api/mail/projects/{id}/request-domains
PUT   /api/mail/projects/{id}/request-domains
POST  /api/mail/projects/{id}/rotate-key
GET   /api/mail/emails
GET   /api/mail/emails/{id}
GET   /api/mail/emails/by-project
```

Scopes:

```text
mail.projects.read
mail.projects.write
mail.credentials.rotate
mail.emails.read
mail.usage.read
```

Normal senders continue to use the independent project API-key path:

```text
POST /v1/emails
Authorization: Bearer mw_live_...
```

---

## 17. Strict BFF facade rule

BFF is deny-by-default.

Do not implement:

```text
/api/<service>/proxy/*
/api/<service>/{arbitrary-path}
```

Each route must explicitly define:

```text
HTTP method
public path
downstream path
required delegated scope
body limits
recent-WebAuthn requirement
```

Unknown routes return 404.

---

## 18. Adding a feature to an existing service

For a new Mail endpoint, for example:

```text
POST /admin/api/projects/{id}/verify-domain
```

implement:

```text
MW Mail
  - business logic
  - admin endpoint
  - scope enforcement
  - audit

MW BFF
  - explicit route
  - exact downstream mapping
  - delegated scope
  - step-up decision
  - bounded request/response handling

MW Web Admin
  - API client
  - UI/workflow
  - step_up_required handling
```

If a new scope is introduced, also update the Identity/BFF application scope configuration and E2E coverage.

Business logic remains in MW Mail.

---

## 19. Adding a new service

For `mw-storage`:

```text
1. MW Storage
   - implement admin API
   - define storage.* scopes
   - introspect Identity delegated tokens
   - require aud=mw-storage
   - enforce route scope
   - audit human sub

2. Identity
   - register resource application
   - client_id=mw-storage
   - audience=mw-storage
   - securely hand introspection secret to service

3. BFF Identity application
   - allow audience mw-storage
   - allow required storage.* scopes

4. BFF
   - add bounded client
   - add strict /storage/* facade
   - route -> scope mapping
   - WebAuthn gates for sensitive actions

5. MW Web Admin
   - add module calling /api/storage/*

6. E2E
   - correct audience succeeds
   - wrong audience fails
   - missing scope fails
   - Identity outage fails closed
   - unmapped route returns 404
   - browser receives no delegated token
```

---

## 20. Local frontend development

Recommended:

```text
Browser: http://localhost:5173
    -> Vite proxies /api
    -> Gateway :3050 with Host admin.mattediworks.com
    -> BFF
```

Identity remains browser-accessible at the configured localhost Identity origin for local WebAuthn testing.

Frontend uses relative URLs:

```text
/api/auth/...
/api/identity/...
/api/oauth/...
/api/mail/...
```

Do not point browser code directly at BFF `:7200`.

A dedicated local OIDC client should use:

```text
redirect_uri=http://localhost:5173/api/auth/callback
BFF_PUBLIC_BASE_URL=http://localhost:5173
```

---

## 21. SQLite

Identity and BFF own independent SQLite databases.

Required invariants:

```text
PRAGMA journal_mode = WAL
PRAGMA foreign_keys = ON
PRAGMA busy_timeout = 5000
```

Do not share their database files.

Named Docker volumes are persistent state.

Never use:

```text
docker compose down -v
```

during normal deployments or troubleshooting.

---

## 22. Backup and restore

Step 13 uses SQLite-aware online snapshots.

```powershell
.\scripts\backup-control-plane.ps1 -EnvFile .env.production
```

Backup contains:

```text
identity.db
bff.db
manifest.json
```

The manifest records sizes and SHA-256 hashes and explicitly states that secrets are not included.

Required external material includes at least:

```text
IDENTITY_SIGNING_KEYS
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS
IDENTITY_TOTP_SECRET_KEYS
BFF_TRANSACTION_KEYS
BFF_OIDC_CLIENT_SECRET
```

Verify:

```powershell
.\scripts\verify-step-13-backup.ps1 `
  -BackupDirectory <backup> `
  -EnvFile .env.production
```

Validation-only restore:

```powershell
.\scripts\restore-control-plane.ps1 `
  -BackupDirectory <backup> `
  -EnvFile .env.production `
  -ValidateOnly
```

Destructive restore requires explicit confirmation and first creates a safety backup.

Never delete named volumes as a restore technique.

---

## 23. Operational hardening

Step 13 adds the production gate:

- `/health`;
- SQLite-aware `/ready`;
- normalized `X-Request-ID`;
- request correlation;
- path-only access logging;
- no sensitive query-string logging;
- gateway abuse controls;
- trusted forwarding-header normalization;
- bounded request/response bodies;
- bounded HTTP timeouts;
- audit retention;
- expired-state cleanup;
- signing-key retired retention;
- production config validation;
- non-root containers;
- read-only root filesystem where supported;
- dropped capabilities;
- `no-new-privileges`;
- bounded tmpfs;
- backup directories excluded from Git and Docker build context.

Baseline audit retention:

```env
IDENTITY_AUDIT_RETENTION_DAYS=180
```

---

## 24. Health vs readiness

```text
/health -> process is alive
/ready  -> service can perform normal work, including persistence
```

Do not use health as a substitute for readiness.

---

## 25. Logging and request IDs

Safe logs may include:

```text
request_id
method
path
status
duration
service
human sub where applicable
```

Do not log:

```text
passwords
session tokens
CSRF tokens
authorization codes
OIDC state/nonce
delegated tokens
client secrets
recovery capabilities
TOTP secrets
WebAuthn challenge/credential blobs
sensitive raw query strings
```

Audit and diagnostic logging are separate concerns.

---

## 26. Production secret placement

Secrets are runtime/deployment configuration.

They must not be:

- baked into images;
- committed;
- embedded in frontend code;
- stored in ordinary documentation;
- returned through APIs except explicit one-time issuance operations.

A `.env` file is only one injection mechanism. StackPort or another production secret-management mechanism may provide the same runtime variables.

---

## 27. Security invariants

v1.0 must preserve:

1. Human identity is `issuer + sub`.
2. Email is not a permanent identity foreign key.
3. Browser never receives Identity access tokens.
4. Browser never receives delegated tokens.
5. Downstream services never receive user passwords.
6. Downstream services never receive Identity signing private keys.
7. Delegated tokens are opaque and short-lived.
8. Delegated tokens are audience-bound.
9. Delegated scopes are narrowed.
10. Resource servers introspect only their own audience.
11. Failed introspection fails closed.
12. Business logic remains in the owning service.
13. BFF remains explicit, never a generic proxy.
14. Sensitive operations require recent WebAuthn where configured.
15. TOTP cannot satisfy explicit WebAuthn step-up.
16. Recovery remains operator-controlled.
17. Ordinary applications require explicit assignments.
18. Project/service credentials remain independent from human federation.
19. Raw credentials are absent from logs/audit.
20. Keyrings remain external to DB backups.
21. Production named volumes are never casually deleted.

---

## 28. Testing baseline

```powershell
go -C .\mw-identity test ./...
go -C .\mw-identity vet ./...

go -C .\mw-bff test ./...
go -C .\mw-bff vet ./...

docker compose config
```

For BFF/downstream changes test:

```text
session
CSRF
explicit route mapping
delegated scope
audience
downstream success
wrong/missing scope denial
unmapped route 404
step-up if sensitive
no bearer exposure
audit attribution
```

For Identity changes test:

```text
positive flow
disabled user/app
assignment denial
one-time tokens
wrong audience
wrong scope
keyring persistence
revocation
audit safety
```

Real browser validation remains required for WebAuthn changes.

---

## 29. v1.0 completion state

The staged implementation has completed the backend/security roadmap through:

```text
STEP 12 E2E PASS
STEP 13 API/OPS PASS
```

Destructive Step 13 operational rehearsals remain deliberate operator gates rather than hidden routine automation.

The normal administrative path is now:

```text
MW Web Admin
    -> Gateway
    -> MW BFF
    -> MW Identity / delegated service
```

This is the baseline architecture for future MW Control Plane development.

---

## 30. v1.1: MQTT module

v1.1 adds MQTT broker administration as a third downstream service. The v1.0 contracts
above are unchanged: OAuth and Mail behave exactly as before.

- **Service:** `mqtt-admin`, in the MQTT broker's own Stackport project, reached on its own
  domain like MW OAuth and MW Mail. It validates delegated tokens by authenticated
  introspection (client and audience `mw-mqtt`) and applies changes to Mosquitto through
  Dynamic Security with its own narrowly scoped broker user.
- **Facade:** `/api/mqtt/*` in the BFF, explicit routes only, one delegated scope per
  route (`mqtt.clients.read|write|delete`, `mqtt.credentials.rotate`,
  `mqtt.roles.read|write|delete`, `mqtt.server.read`). Create, re-enable, password rotation,
  delete, and every change to roles or role assignments require fresh WebAuthn; disabling a
  client does not, so it stays a fast kill switch.
- **Optional:** the facade is disabled unless `BFF_MQTT_BASE_URL` is set, so a v1.0
  deployment is unaffected until it opts in.
- **Console:** an `MQTT` group with clients, broker status and documentation pages. The
  overview shows an MQTT module card that is online only while the broker connection is up.
- **Roles** can be created, edited and deleted in the console, and assigned to clients. The
  broker's own roles are reserved, no rule can grant the control channel, and deleting a role
  that clients still hold needs explicit confirmation.

Enabling it is described in `dev_docs/stackport-onboarding.md`, section 3b.
