# MW Identity — Project Plan

> Historical implementation/planning snapshot. The [2026-11-04 clean v1 migration decision](federation-v1-clean-migration.md) supersedes compatibility, application-wide Strong and per-application signup behavior described below.

## Responsibility

`mw-identity` is the trust root for MattediWorks application identity.

It operates as:

- Identity Provider (IdP);
- OpenID Provider (OP);
- OAuth Authorization Server (AS);
- user/admin credential authority;
- application/client registry;
- issuer of identity and delegated access assertions.

It serves both administrators and ordinary application users.

## Data ownership

Identity owns:

```text
users
user status
basic identity profile
password credentials where enabled
WebAuthn credentials
TOTP secrets later
recovery state
identity sessions
applications / OIDC clients
redirect URI allowlists
application assignments
coarse roles/entitlements
authorization grants
authorization codes
refresh tokens if introduced
signing keys/key versions
authentication/security audit
```

It does not own application-specific business profiles.

WOI, for example, keeps its own user/domain row and links it to the stable
Identity subject.

## Initial Go structure

```text
mw-identity/
├── cmd/
│   ├── server/
│   └── create-superadmin/
├── internal/
│   ├── config/
│   ├── httpapi/
│   ├── store/
│   │   └── sqlite/
│   ├── auth/
│   ├── oidc/
│   ├── applications/
│   ├── assignments/
│   ├── tokens/
│   ├── signing/
│   ├── webauthn/
│   ├── audit/
│   └── portal/
├── migrations/
└── web/
    └── static/
```

Exact package boundaries may evolve, but persistence, protocol, HTTP, and
authentication logic should not collapse into one package.

## Identity portal

The first portal is intentionally minimal.

It only needs enough HTML/JavaScript for:

- password login;
- WebAuthn registration/authentication when implemented;
- consent/authorization if required;
- TOTP later;
- recovery later;
- basic success/error pages.

No design system is required initially.

## OIDC application model

Conceptual app/client record:

```text
applications
  id
  name
  client_id
  enabled
  archived_at nullable
  application_type
  authentication_policy
  allow_public_signup
  internal                 behind the MW BFF, no user data; exempt from the callback
  delete_callback_uri nullable, but required unless internal
  created_at
  updated_at
```

`delete_callback_uri` is mandatory for every non-internal application on create
and edit (`delete_callback_uri` stays nullable only for internal and legacy
rows). Internal applications cannot allow public signup. See the deletion
contract in `docs/MW-Identity-application-integration.md`.

Related normalized configuration:

```text
application_redirect_uris
application_post_logout_redirect_uris
application_scopes
application_claims
application_audiences
application_assignments
```

Confidential clients store only a secure client secret hash or later an
asymmetric client-authentication registration.

Raw client secrets are shown once on create/rotate.

Redirect URIs use exact allowlists.

## Planned protocol endpoints

Initial surface:

```text
GET  /.well-known/openid-configuration
GET  /.well-known/jwks.json

GET  /oauth/authorize
POST /oauth/token
GET  /oauth/userinfo      # when needed
POST /oauth/revoke        # later if needed
```

Authorization Code + PKCE is the primary browser/RP flow.

## Token principles

Every token is restricted to its intended use.

At minimum validate:

```text
iss
aud
exp
sub
scope / permissions where applicable
```

A token for:

```text
aud = mw-oauth
```

must not be accepted by MW Mail.

Signing keys are versioned/rotatable. Public verification material is exposed
through JWKS. Private signing keys never leave Identity.

## Delegation to downstream services

Final Admin BFF behavior:

```text
Admin browser session
  -> BFF
  -> Identity delegated/token-exchange operation
  -> short-lived token aud=mw-oauth or aud=mw-mail
  -> downstream service
```

The delegated token preserves the human subject and enough authentication
context for authorization/audit.

The exact token-exchange implementation is a later gated step after base OIDC
login works end-to-end.

## Authentication policies

Identity should support app-specific authentication strength.

Example:

```text
WOI:
  password OR passkey

MattediWorks Admin:
  passkey required eventually
```

Initial implementation can start with password and then optional WebAuthn.
Later phases add mandatory WebAuthn for Admin, TOTP fallback, recent-auth
requirements, and manual recovery.

## Bootstrap

The first superadmin must not depend on an already-working Admin plane.

Provide:

```text
mw-identity/cmd/create-superadmin
```

Initial responsibilities:

- create first Identity account;
- secure password hash;
- privileged role/assignment;
- audit.

## Identity administration

Identity Admin API eventually manages:

```text
users
applications
assignments
roles/entitlements
sessions
credentials/status
signing-key status
audit
```

Example app management:

```text
GET    /admin/api/applications
POST   /admin/api/applications
GET    /admin/api/applications/{id}
PATCH  /admin/api/applications/{id}
POST   /admin/api/applications/{id}/rotate-secret
POST   /admin/api/applications/{id}/disable
POST   /admin/api/applications/{id}/enable
DELETE /admin/api/users/{id}
```

Ordinary clients must not self-edit security-sensitive registration settings.

For the relying-party login flow, user claims, public enrollment, connected applications, and the signed deletion callback contract, see [MW-Identity application integration](../MW-Identity-application-integration.md).

## Security invariants

Do not persist/log plaintext:

- passwords;
- client secrets;
- TOTP secrets without encryption;
- bearer tokens;
- ID/refresh tokens;
- authorization codes;
- WebAuthn ceremony secrets.

Use:

- secure random opaque IDs;
- one-time short-lived authorization codes;
- PKCE;
- exact redirect matching;
- secure HttpOnly Identity session cookies;
- CSRF for cookie-authenticated mutations;
- authentication/token rate limits;
- structured audit;
- key versioning/rotation.

## Account linking

Identity supplies a stable `sub`.

The relying app owns the mapping between that `sub` and its existing local
user.

Identity may know whether a user is allowed to use an app, but it does not
need to know the app's internal user primary key.

See `account-linking.md`.

## Existing service compatibility

Existing MW OAuth/Mail local auth remains usable during migration:

```text
local -> hybrid -> federated
```

The service-local path remains valuable for isolated testing/debugging and
explicit break-glass use.
