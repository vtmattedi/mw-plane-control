# Step 04 — Authorization Code + PKCE

Step 04 implements the first complete MW Identity OIDC Authorization Code
flow:

```text
GET  /oauth/authorize
POST /oauth/token
GET  /oauth/userinfo
```

## Security contract

- Authorization Code only.
- PKCE is mandatory.
- Only `S256` is accepted.
- `state` is required by MW Identity policy.
- Redirect URI matching is exact.
- Unknown clients or invalid redirect URIs are never redirected to.
- Authorization codes are random, short-lived, stored only as SHA-256 hashes,
  and consumed once.
- Confidential clients authenticate at the token endpoint.
- Bad client authentication or bad PKCE does not consume an otherwise valid
  authorization code.
- Access tokens are opaque random bearer tokens stored only as SHA-256 hashes.
- ID tokens are RS256 JWTs signed by the active Step 03 Identity key.
- No refresh tokens are issued in Step 04.

Default lifetimes:

```text
authorization code: 5 minutes
access token:       60 minutes
ID token:            5 minutes
```

ID-token claims currently include `iss`, `sub`, `aud`, `exp`, `iat`,
`auth_time`, optional `nonce`, plus `email`/`name` when their scopes were
granted.

The stable Identity user ID is the OIDC `sub`.

## Login continuation

An unauthenticated authorization request is sent to the Identity login page
with a relative `return_to`. After successful login, the original
authorization request resumes.

`return_to` is restricted to local absolute-path references. External
absolute URLs, network-path references (`//host`), and backslash forms are
rejected.

## Consent / assignments

Step 04 intentionally does not add a consent UI or application assignment
policy. During the first-party bootstrap phase, an enabled registered client
may authorize an enabled authenticated Identity user.

Assignments and Identity administration are introduced in later gated steps.

## E2E

Run:

```powershell
.\scripts\e2e-step-04.ps1
```

It checks the browser authorization redirect, Identity login continuation,
state, PKCE, confidential-client authentication, one-time code consumption,
RS256 ID-token verification against JWKS, userinfo, and Step 03/02 regression.

Completion marker:

```text
STEP 04 E2E PASS
```

## Subsequent contract revision (2026-10-27)

The historical bootstrap consent omission above is superseded by explicit external-client consent, version-fenced credentials, non-destructive disconnect and normal max_age handling. See ../MW-Identity-application-integration.md and ../dev_docs/federated-signin-rollout.md. Internal clients retain a documented trusted bootstrap; explicit disconnect still revokes it.
