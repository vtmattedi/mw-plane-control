# Step 06 — Identity administration through Admin/BFF

> Historical implementation/planning snapshot. The [2026-11-04 clean v1 migration decision](../dev_docs/federation-v1-clean-migration.md) supersedes compatibility, application-wide Strong and per-application signup behavior described below.

## Goal

MW Identity now manages its own runtime application registry, assignments,
users, privileged Identity-admin permissions, and audit through the shared
Admin plane.

The browser never receives the Identity bearer token.

```text
Admin browser
  -> /api/identity/*
  -> MW BFF local admin session + CSRF
  -> decrypt short-lived Identity access token
  -> private Docker request
  -> MW Identity /admin/api/*
```

## Identity Admin API

Protected resource audience:

```text
mw-identity-admin
```

Required OAuth scope:

```text
identity.admin
```

Routes:

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

GET    /admin/api/applications/{id}/assignments
PUT    /admin/api/applications/{id}/assignments/{user_id}
DELETE /admin/api/applications/{id}/assignments/{user_id}

GET    /admin/api/audit
```

## BFF surface

The BFF exposes the same module under:

```text
/api/identity/*
```

The gateway strips `/api/`, so BFF sees `/identity/*`.

GET requests require a valid BFF admin session.

Mutating requests require both the valid session and the current
`X-MW-CSRF` token.

The proxy has bounded request/response bodies, bounded HTTP timeout, does not
follow redirects, and forwards only a small allowlist of headers.

## Access token handling

Step 05 deliberately discarded the Identity access token after login.

Step 06 keeps it because the BFF must call the Identity Admin resource on
behalf of the signed-in administrator.

It is never persisted plaintext.

The existing versioned BFF AES-256-GCM keyring protects it in
`bff_admin_sessions`, with AAD bound to the BFF session ID.

The browser only receives the opaque BFF session cookie.

The local BFF session expiry is capped by the Identity access-token expiry.
Without refresh-token support yet, the BFF never keeps an apparently valid
Identity-admin session after its delegated bearer token has expired.

## Audience restriction

A BFF application used for Identity administration must be registered with:

```text
scope:    identity.admin
audience: mw-identity-admin
```

The Identity token endpoint issues an access token with that audience only
when the client is configured for it.

The ID token still uses the OIDC client ID as its `aud`, independently of the
access-token resource audience.

## Assignments

Assignments bind a stable MW Identity subject to an application:

```text
application_id + user_id(sub) + role
```

Roles in Step 06:

```text
user
admin
superadmin
```

This is also the first persistence foundation for later "link existing user
to MW Identity" workflows: applications reference the stable Identity user
ID, not email.

OIDC authorization now requires an assignment to the target application.

The only bootstrap exception is the global Identity superadmin authenticating
to an application that both requests `identity.admin` and is registered for
the `mw-identity-admin` audience. The root account therefore does not gain an
implicit login assignment to unrelated applications such as WOI.

## Identity-admin permissions

`superadmin`:

```text
identity.applications.read
identity.applications.write
identity.applications.rotate_secret
identity.users.read
identity.assignments.read
identity.assignments.write
identity.audit.read
```

`admin`:

```text
identity.applications.read
identity.applications.write
identity.applications.rotate_secret
identity.users.read
identity.assignments.read
identity.audit.read
```

An `admin` cannot modify assignments. That prevents the basic admin role from
granting itself or another identity a stronger role.

The original bootstrap `identity_users.is_superadmin` remains the operator
root and maps to the `superadmin` permission set.

## Application lifecycle

`client_id` and application type are immutable after creation in Step 06.

Editable configuration:

```text
name
authentication_policy
exact redirect URIs
exact post-logout redirect URIs
scopes
claims
audiences
```

Lifecycle:

```text
enabled
  -> disabled
  -> enabled

enabled/disabled
  -> archived
```

Archived applications cannot be re-enabled or have their secret rotated.

Hard delete is intentionally absent.

## Client-secret rotation

Confidential-client secret rotation returns a fresh high-entropy secret once.

Only its SHA-256 hash is persisted by Identity.

The previous secret becomes invalid immediately.

## Audit

Identity-admin mutations append events to the existing Identity audit log:

```text
identity.application.created
identity.application.updated
identity.application.disabled
identity.application.enabled
identity.application.archived
identity.application.secret_rotated
identity.assignment.upserted
identity.assignment.deleted
```

Audit records include the acting Identity user and remote IP where available.
Secrets and bearer tokens are never audit metadata.

## E2E checkpoint

Run:

```powershell
.\scripts\e2e-step-06.ps1
```

The E2E performs a real BFF/Identity login and then, through
`/api/identity/*` only:

```text
login with Identity
-> check Identity-admin permissions
-> list users
-> create OIDC app
-> replace exact redirect URI
-> disable
-> enable
-> rotate secret
-> create/list/delete assignment
-> inspect audit
-> archive
```

It also verifies that a mutating Identity-admin request without BFF CSRF is
rejected.

Completion marker:

```text
STEP 06 E2E PASS
```
