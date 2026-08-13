# MW BFF — Project Plan

## Responsibility

`mw-bff` is the Backend-for-Frontend for the privileged
`admin.mattediworks.com` SPA.

The browser normally communicates only with:

```text
https://admin.mattediworks.com
```

Gateway behavior:

```text
/      -> static Admin SPA
/api/* -> mw-bff
```

The BFF communicates server-to-server with Identity and downstream service
admin APIs.

## What the BFF owns

The BFF owns:

- Admin browser sessions;
- secure session cookie;
- CSRF;
- login initiation;
- OIDC callback processing;
- authorization-code exchange;
- ID-token verification;
- server-side token lifecycle;
- delegated downstream token acquisition;
- module API routing;
- downstream failure translation;
- BFF audit/observability;
- Admin logout.

The BFF does not own:

- Identity credentials;
- OAuth project business logic;
- Mail business logic;
- StackPort operations;
- direct Identity database access.

## Initial Go structure

```text
mw-bff/
├── cmd/
│   └── server/
├── internal/
│   ├── config/
│   ├── httpapi/
│   ├── session/
│   ├── csrf/
│   ├── identityclient/
│   ├── delegation/
│   ├── modules/
│   │   ├── identity/
│   │   ├── oauth/
│   │   └── mail/
│   ├── store/
│   │   └── sqlite/
│   └── audit/
└── migrations/
```

## Browser session

After OIDC login, the BFF creates a server-side opaque Admin session.

Planned cookie:

```text
HttpOnly
Secure in production
SameSite=Strict where compatible with completed callback flow
Path=/
host-only for admin.mattediworks.com
```

Store only a secure hash of the raw session token.

CSRF is required for cookie-authenticated mutations.

The SPA must not keep long-lived OAuth tokens in browser storage.

## OIDC login flow

```text
Browser
  GET /api/auth/login
      |
      v
BFF
  state + PKCE
  redirect to Identity
      |
      v
Identity
  login
  authorization code
      |
      v
BFF /api/auth/callback
  validate state
  exchange code
  validate ID token
  create BFF session
      |
      v
Browser -> Admin SPA
```

## Downstream modules

BFF namespaces:

```text
/api/identity/*
/api/oauth/*
/api/mail/*
```

Example:

```text
Browser:
POST /api/oauth/projects/12/rotate-callback-secret
```

BFF:

1. validates Admin session;
2. validates CSRF;
3. checks current Identity entitlement;
4. obtains/uses delegated token for `aud=mw-oauth`;
5. calls MW OAuth admin API;
6. returns safe result;
7. preserves original actor for audit.

The downstream service remains authoritative for business/admin rules.

## Delegated tokens

A generic service credential that makes all actions look like:

```text
actor = mw-bff
```

is not sufficient.

Final delegated authorization must preserve:

```text
human subject
target audience
limited scopes/permissions
authentication method/context
authentication time when needed
```

The likely implementation is an Identity token-exchange/delegation endpoint
added after basic OIDC is working.

## Identity administration through BFF

The shared Admin includes an Identity module:

```text
Browser
-> /api/identity/applications
-> BFF
-> Identity Admin API
```

The browser never receives privileged Identity admin credentials directly.

First-superadmin bootstrap remains operator-driven.

## Service-local fallback

BFF does not replace standalone development paths inside MW OAuth/Mail.

Services migrate through:

```text
ADMIN_AUTH_MODE=local
ADMIN_AUTH_MODE=hybrid
ADMIN_AUTH_MODE=federated
```

BFF uses the federated path.

## Storage

BFF may use SQLite initially for:

- Admin sessions;
- OIDC state/PKCE transactions;
- CSRF/session metadata;
- limited BFF audit;
- short-lived token metadata where necessary.

BFF never reads Identity's SQLite database directly.

## Security invariants

- No access/refresh tokens in localStorage/sessionStorage.
- No downstream privileged credential in frontend JS.
- No Identity signing private keys in BFF.
- No direct Identity DB access.
- Host-only Admin session cookie.
- CSRF for cookie-authenticated mutations.
- OIDC state + PKCE.
- Strict issuer/callback validation.
- Audience validation on downstream tokens.
- Safe timeouts and bounded response bodies.
- No bearer tokens/cookies/auth codes in logs.
- Preserve original actor identity in audit.
