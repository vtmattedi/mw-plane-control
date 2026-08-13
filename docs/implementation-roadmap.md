# MW Plane Control — Implementation Roadmap

## Development rule

Implementation proceeds in gated steps.

Every step must have:

1. explicit goals;
2. implementation changes;
3. automated tests;
4. runtime sanity checks;
5. an end-to-end checkpoint when applicable;
6. backup-safe patching/generation where incremental scripts are used.

Do not begin the next step while the current step has failing tests or an
unexplained runtime result.

Primary engineering focus:

```text
MW Identity + MW BFF
```

Frontend polish is deliberately secondary.

# Step 00 — Bootstrap and architecture freeze

## Goals

- Create monorepo scaffold.
- Freeze project names/boundaries.
- Freeze one-gateway-port topology.
- Establish docs.
- No working authentication yet.

## Output

```text
mw-plane-control/
├── docs/
├── infra/gateway/
├── mw-identity/
├── mw-bff/
└── mw-web-admin/
```

Go directories are scaffolded.

`mw-web-admin` remains only a parent directory.

## Tests

- initializer dry-run;
- initializer refuses unsafe overwrite;
- resulting tree matches expected scaffold.

# Step 01 — Runtime skeleton and health path

## Goals

Bring up the backend/control-plane skeleton end-to-end without auth.

### Identity

Implement:

```text
GET /health
```

plus:

- config loader;
- structured logging;
- graceful shutdown;
- SQLite;
- migration runner;
- build metadata.

### BFF

Implement:

```text
GET /health
```

with the same operational baseline and its own SQLite.

### Gateway

Implement:

- one exposed host port;
- Host routing for Admin and Identity;
- placeholder/static Admin test page;
- `/api/*` to BFF;
- Identity host to Identity.

### Identity portal

Plain HTML:

```text
MW Identity is running.
```

## E2E checkpoint

Through gateway:

```text
Host=admin... /             -> static page
Host=admin... /api/health   -> BFF
Host=identity... /health    -> Identity
```

## Tests

- Go unit tests;
- `go vet`;
- migration tests;
- Compose build;
- gateway routing sanity.

# Step 02 — Identity local account + bootstrap admin

## Goals

Create first usable Identity account and secure Identity web session.

Implement:

- `users`;
- Argon2id password hashing;
- enabled/disabled state;
- `create-superadmin` CLI;
- opaque Identity session;
- CSRF;
- password login/logout;
- minimal HTML login;
- authentication audit.

No OIDC yet.

## E2E checkpoint

```text
create-superadmin
-> login page
-> password login
-> protected Identity page
-> logout
```

## Tests

- password hash/verify;
- session raw token not stored;
- expiry;
- disabled user;
- CSRF;
- cookie policy;
- rate-limit baseline.

# Step 03 — Signing keys, discovery, and app registry

## Goals

Make Identity an identifiable OIDC issuer before login federation.

Implement:

- versioned signing keys;
- JWKS;
- discovery;
- OIDC application/client tables;
- exact redirect URIs;
- scopes/claims/audiences;
- secure client secret generation/hash;
- enable/disable/archive;
- bootstrap creation of first Admin client.

Protocol:

```text
GET /.well-known/openid-configuration
GET /.well-known/jwks.json
```

## E2E checkpoint

- register `mw-admin`;
- read discovery;
- read JWKS;
- verify canonical public issuer.

## Tests

- signing/verification;
- JWKS representation;
- key rotation compatibility;
- exact redirect matching;
- client secret shown once/not stored raw.

# Step 04 — Authorization Code + PKCE

## Goals

Complete first real OIDC authorization flow.

Implement:

```text
GET  /oauth/authorize
POST /oauth/token
```

For first-party Admin:

- Authorization Code;
- PKCE S256;
- short-lived one-time code;
- ID token;
- access token;
- exact redirect validation;
- client authentication where applicable;
- Identity session reuse.

Minimal Identity HTML is enough.

## E2E checkpoint

Use a test OIDC client/script:

```text
authorize
-> login
-> code
-> token exchange
-> verify ID token against JWKS
```

## Tests

- code one-time use;
- expiry;
- wrong redirect;
- wrong PKCE verifier;
- disabled client;
- bad client credential;
- issuer/audience;
- no code/token logging.

# Step 05 — BFF OIDC login

## Goals

Make MW BFF a real OIDC RP/client.

Implement:

```text
GET  /api/auth/login
GET  /api/auth/callback
GET  /api/auth/me
POST /api/auth/csrf
POST /api/auth/logout
```

BFF:

- creates state/PKCE transaction;
- redirects to Identity;
- exchanges code server-side;
- validates ID token;
- creates opaque Admin browser session;
- keeps token material server-side;
- adds CSRF.

Use the basic test Admin frontend.

## E2E checkpoint

```text
Admin test page
-> Sign in
-> Identity password login
-> BFF callback
-> Admin session
-> /api/auth/me
-> logout
```

This is the first major end-to-end milestone.

## Tests

- state mismatch/replay;
- callback expiry;
- issuer/audience/signature;
- session token hash-at-rest;
- CSRF;
- logout;
- Identity unavailable behavior.

# Step 06 — Identity administration through Admin/BFF

## Goals

Manage Identity itself through the shared Admin plane.

Identity Admin API:

- list/create/update apps;
- disable/enable/archive;
- rotate client secret;
- list users;
- basic assignments;
- privileged Identity-admin permissions;
- audit.

BFF:

```text
/api/identity/*
```

module/controller.

Minimal test UI is sufficient.

## E2E checkpoint

From Admin:

```text
login with Identity
-> create OIDC app
-> set exact redirect URI
-> disable
-> enable
-> rotate secret
-> inspect audit
```

# Step 07 — Optional WebAuthn at Identity

## Goals

Move optional passkeys into the true Identity boundary.

Implement:

- stable Identity WebAuthn user handle;
- multiple credentials;
- discoverable credentials;
- `userVerification=required`;
- resident/discoverable credential required;
- attestation none;
- encrypted credential/session data;
- short-lived single-use ceremonies;
- registration;
- passkey login;
- credential list/remove;
- audit.

Identity portal remains minimal.

## E2E checkpoint

```text
password login
-> register Windows Hello/passkey
-> logout
-> login with passkey
```

BFF does not implement WebAuthn itself.

# Step 08 — Existing account linking

## Goals

Provide reusable linking for apps such as WOI.

Proof flow:

```text
existing local account
-> Link MattediWorks Identity
-> OIDC
-> validated sub
-> local external_identity mapping
```

Also test:

```text
Sign in with MW Identity
-> sub not linked
-> require existing app authentication before linking
```

Never auto-link solely by email match.

## E2E checkpoint

- link;
- login via link;
- duplicate subject rejected;
- safe unlink;
- disabled Identity/app assignment behavior.

See `account-linking.md`.

# Step 09 — Delegated downstream service tokens

## Goals

Let BFF call downstream services on behalf of the human administrator.

Design/implement Identity delegation or token exchange.

Target token properties:

```text
sub = human identity
aud = exact service
scope/permissions = limited
short lifetime
auth method/context retained
```

Implement BFF delegation client/cache.

## E2E checkpoint

Use a protected mock resource server first:

```text
Admin user
-> BFF
-> token aud=test-rs
-> mock RS
-> human actor preserved
```

Then federate real services.

# Step 10 — MW OAuth federation

## Goals

Adapt MW OAuth without breaking standalone development.

Implement:

```text
ADMIN_AUTH_MODE=local|hybrid|federated
ALLOW_LOCAL_ADMIN_AUTH_IN_PRODUCTION=false
```

Federated path validates:

- Identity signature/JWKS;
- issuer;
- audience `mw-oauth`;
- expiry;
- permissions;
- later strong-auth context.

BFF OAuth module calls existing admin operations.

## E2E checkpoint

```text
Admin Identity login
-> BFF
-> MW OAuth
-> list/create/configure project
-> rotate credential
-> MW OAuth audit identifies human actor
```

Also verify local mode still works.

# Step 11 — MW Mail federation

Repeat controlled migration:

```text
local -> hybrid -> federated
```

Verify shared Admin Mail module end-to-end.

Business logic stays in MW Mail.

# Step 12 — Strong auth, TOTP, recovery

After federation is stable:

- Admin requires WebAuthn;
- ordinary apps can choose their own policy;
- TOTP fallback;
- recent-auth/step-up;
- manual recovery;
- session revocation;
- assurance context.

High-risk examples:

```text
Identity client-secret rotation
Identity role/admin changes
OAuth callback-secret rotation
OAuth callback allowlist changes
MFA/passkey changes
```

# Step 13 — Hardening and operational readiness

Implement/verify:

- signing key rotation;
- client credential rotation;
- backup/restore;
- audit retention;
- rate limits;
- abuse controls;
- token revocation strategy;
- metrics/request IDs;
- no-sensitive-query logging;
- trusted proxy headers;
- health/readiness;
- Docker hardening;
- SQLite WAL/FKs/busy timeout;
- disaster recovery runbook;
- manual Identity recovery runbook;
- production config validation.

Only then should the shared control plane become the normal production admin
path.

# Frontend parallel track

`mw-web-admin` evolves independently while backend Steps 01–09 proceed.

Backend development only requires a minimal browser harness.

The final preferred MattediWorks frontend stack can be installed later without
changing the Identity/BFF protocol contracts.
