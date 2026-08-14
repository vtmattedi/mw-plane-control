# MW Plane Control — Project Architecture

## Purpose

`mw-plane-control` is the MattediWorks application identity and administrative
control plane.

Its primary goals are to:

1. provide a reusable MattediWorks Identity Provider for MattediWorks apps;
2. provide one MattediWorks Admin interface for application administration;
3. let the Admin BFF act on behalf of an authenticated administrator when
   calling service-specific admin APIs;
4. keep service business/admin logic inside each service;
5. keep infrastructure control such as StackPort outside this trust plane.

StackPort remains a separate privileged infrastructure plane. A compromise of
application identity/admin must not automatically grant Docker, Nginx,
firewall, deployment, or host control.

## Projects

```text
mw-plane-control/
├── docs/
├── infra/
│   └── gateway/
├── mw-identity/
├── mw-bff/
├── mw-web-admin/
├── docker-compose.yml       # added in a later implementation step
└── .env.example             # added in a later implementation step
```

### MW Identity

`mw-identity` is the Identity Provider (IdP), OpenID Provider (OP), and OAuth
Authorization Server (AS).

It owns:

- users;
- password credentials where enabled;
- WebAuthn/passkeys;
- TOTP later;
- recovery;
- Identity sessions;
- OIDC applications/clients;
- application assignments;
- coarse application roles/entitlements;
- authorization grants;
- signing keys/JWKS;
- authentication/security audit.

It does not own application business data.

### MW BFF

`mw-bff` is the Backend-for-Frontend for `admin.mattediworks.com`.

It owns:

- the Admin browser session;
- CSRF protection;
- OIDC client flow against MW Identity;
- server-side token handling;
- downstream module routing;
- acting on behalf of the authenticated administrator.

It does not implement OAuth-project, Mail-project, or other service business
logic.

### MW Web Admin

`mw-web-admin` is the Vite/React SPA source for
`admin.mattediworks.com`.

It is a static build artifact. Production Nginx serves its compiled `dist/`.
It does not require its own runtime container.

## Runtime topology

Target production shape:

```text
                         Internet
                            |
                            v
                  StackPort / host Nginx
                TLS + public domain routing
                            |
                            | one loopback host port
                            v
                 +------------------------+
                 | control-plane gateway  |
                 | Nginx container        |
                 |                        |
                 | serves Admin SPA dist  |
                 +-----------+------------+
                             |
                 +-----------+-----------+
                 |                       |
                 v                       v
           +-----------+           +-------------+
           |  MW BFF   |           | MW Identity |
           |    Go     |           |     Go      |
           +-----+-----+           +-------------+
                 |
          +------+------+
          |             |
          v             v
      MW OAuth       MW Mail
    separate stack  separate stack
```

Runtime containers in this Compose project:

1. gateway Nginx;
2. `mw-bff`;
3. `mw-identity`.

`mw-web-admin` is compiled during the gateway image build and copied into the
Nginx image.

Only the gateway publishes a host port. BFF and Identity stay on the private
Docker network.

## Public hosts

Planned public hosts:

```text
https://admin.mattediworks.com
https://identity.mattediworks.com
```

Gateway routing:

```text
Host: admin.mattediworks.com
    /api/*  -> mw-bff
    /*      -> static Admin SPA

Host: identity.mattediworks.com
    /*      -> mw-identity
```

The outer StackPort-managed Nginx forwards both domains to the same loopback
gateway port. The inner gateway distinguishes them by `Host`.

Outer proxy must preserve at least:

```text
Host
X-Real-IP
X-Forwarded-For
X-Forwarded-Proto
```

The canonical Identity issuer remains:

```text
https://identity.mattediworks.com
```

even though internal Docker traffic reaches Identity via a private service
name.

## Authentication traffic versus normal admin traffic

Identity is **not** in the data path for every admin request.

Login:

```text
Browser
  -> Admin BFF
  -> redirect to MW Identity
  -> authenticate
  -> OIDC authorization code
  -> Admin BFF callback
  -> BFF exchanges code
  -> BFF creates browser session
```

Normal administration:

```text
Browser
  -> Admin BFF
  -> downstream service admin API
```

Identity is used when authentication, token issuance/exchange, step-up,
session/authorization refresh, or other identity operations are required.
Downstream APIs should normally verify signed tokens locally.

## Acronyms and vocabulary

- **IdP** — Identity Provider. MW Identity.
- **OIDC** — OpenID Connect.
- **OAuth** — authorization framework used for delegated tokens.
- **OP** — OpenID Provider. MW Identity.
- **RP** — Relying Party. Admin, WOI, future apps.
- **AS** — Authorization Server. MW Identity.
- **RS** — Resource Server. MW OAuth/Mail admin APIs.
- **BFF** — Backend for Frontend. MW BFF.
- **SSO** — Single Sign-On.
- **PKCE** — Proof Key for Code Exchange.
- **JWKS** — JSON Web Key Set.
- **MFA** — Multi-Factor Authentication.
- **WebAuthn** — passkeys / Windows Hello / security keys.
- **TOTP** — time-based authenticator code.

## Authorization boundaries

Authentication answers:

```text
Who are you?
```

MW Identity owns this.

Broad application entitlement answers:

```text
Can this identity use this application?
```

Identity can own this.

Fine-grained service authorization answers:

```text
Can this identity perform this operation?
```

The downstream service owns the meaning and enforcement.

Example:

```text
Identity:
    mw-oauth.admin

MW OAuth:
    oauth.projects.read
    oauth.projects.write
    oauth.credentials.rotate
```

Identity must not become a central copy of every service's business
authorization rules.

## BFF acting on behalf of the administrator

Example:

```text
Browser
  POST /api/oauth/projects/12/rotate-callback-secret
      |
      v
MW BFF
  validates session + CSRF
  resolves current identity
  obtains audience-limited delegated token
      |
      v
MW OAuth
  validates issuer/signature/audience/permission
  performs operation
  audits the original human actor
```

A downstream audit must not collapse all actions into a generic `mw-bff`
service identity.

## Direct/local service authentication

Existing services remain independently developable/debuggable.

Planned service setting:

```text
ADMIN_AUTH_MODE=local
ADMIN_AUTH_MODE=hybrid
ADMIN_AUTH_MODE=federated
```

- `local` — service-local login only.
- `hybrid` — local + MW Identity federation.
- `federated` — MW Identity-backed admin only.

Production safety:

```env
ALLOW_LOCAL_ADMIN_AUTH_IN_PRODUCTION=false
```

Production with `local` or `hybrid` should refuse startup unless break-glass
local auth is explicitly enabled.

## Identity for admins and general application users

MW Identity is not admin-only.

Example:

```text
usr_123
├── MattediWorks Admin -> superadmin
├── WOI                -> user
└── Other App          -> no access
```

Applications keep their own domain user/profile rows and link them to the
stable Identity `sub`.

This supports:

```text
Sign in with MattediWorks
```

for apps such as WOI without moving application business profiles into
Identity.

See `account-linking.md`.

## Identity application management

Identity needs its own admin API/UI for:

- applications/clients;
- exact redirect URI allowlists;
- post-logout URI allowlists;
- scopes/claims/audiences;
- client credentials;
- enable/disable/archive;
- assignments;
- roles/entitlements;
- sessions;
- audit.

Normal app removal should usually disable/archive rather than hard-delete,
preserving audit history.

The first Identity superadmin is bootstrapped by an operator CLI.

## WebAuthn ownership

Long-term WebAuthn belongs to Identity:

```text
RP ID:
identity.mattediworks.com

Origin:
https://identity.mattediworks.com
```

Identity therefore needs a minimal browser portal on its own origin for login,
passkey ceremonies, TOTP, recovery, and consent/authorization when required.

For early backend work, this portal can be plain HTML with almost no CSS.

## Databases and secrets

Identity and BFF use independent storage and independent secrets.

Never:

```text
BFF -> open identity.db directly
```

Always use protocol/API boundaries.

Identity signing private keys remain Identity-only. BFF and downstream
resource servers verify Identity-issued tokens using public signing
keys/JWKS.

## StackPort boundary

StackPort remains outside this application control plane.

It may later learn that one Compose stack can have multiple domains pointing
to the same target port, but its privileged infrastructure administration
remains independently protected.

## Initial non-goals

The first implementation does not require:

- SSR;
- React Server Components;
- polished Identity UI;
- public third-party IdP product support;
- LDAP/SCIM;
- StackPort as an Admin module;
- centralization of every service permission;
- immediate passwordless-only enforcement.

The primary focus is the BFF + Identity protocol/security foundation and
end-to-end testability.
