# Step 05 — MW BFF OIDC Login

## Result

`mw-bff` is now a confidential OIDC relying party of MW Identity.

Browser login:

```text
admin browser
  -> GET /api/auth/login
  -> BFF creates state + nonce + PKCE
  -> redirect identity.mattediworks.com/oauth/authorize
  -> Identity password login
  -> authorization code
  -> admin.mattediworks.com/api/auth/callback
  -> BFF claims one-time transaction
  -> BFF exchanges code server-to-server
  -> BFF validates RS256 ID token against Identity JWKS
  -> BFF creates opaque local Admin session
```

Identity is not in the path for normal `/api/auth/me` session reads.

## Public Admin auth API

Through the gateway:

```text
GET  /api/auth/login
GET  /api/auth/callback
GET  /api/auth/me
POST /api/auth/csrf
POST /api/auth/logout
```

The gateway strips `/api/` before forwarding to BFF.

## Network split

BFF distinguishes canonical protocol identity from internal transport:

```env
BFF_IDENTITY_ISSUER=https://identity.mattediworks.com
BFF_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
```

Browser authorization redirects use the canonical public issuer.

Server-to-server token and JWKS calls use the private Docker address.

ID-token `iss` is always validated against the public canonical issuer.

## OIDC transaction storage

BFF persists only short-lived transaction metadata:

```text
SHA-256(state)
SHA-256(nonce)
encrypted PKCE verifier
exact safe Admin return_to
TTL
consumed_at
```

The raw state and nonce are not stored.

The PKCE verifier is encrypted with a versioned AES-256-GCM BFF transaction
keyring:

```env
BFF_TRANSACTION_KEYS=1:<base64url-32-byte-key>
BFF_TRANSACTION_ACTIVE_KEY_VERSION=1
```

Default transaction TTL:

```text
10 minutes
```

State claim/consumption is atomic and single-use.

## ID-token validation

BFF validates:

```text
compact JWS shape
alg = RS256
kid exists in Identity JWKS
RSA signature
iss exact
aud contains BFF client_id
exp
iat
auth_time
nonce hash
sub present
```

Clock skew default:

```text
60 seconds
```

Token/JWKS calls have bounded timeouts, bounded response reads, and redirects
are not followed.

The access token and ID token are not persisted in Step 05. They are used only
during callback completion and then discarded from the application flow.

## Admin browser session

Successful login creates an opaque BFF session.

Cookie:

```text
mw_admin_session
HttpOnly
SameSite=Strict
Path=/
host-only
Secure outside development/local/test
```

SQLite stores only SHA-256(session token).

A separate CSRF token is issued. SQLite stores only SHA-256(CSRF).

Default Admin session TTL:

```text
8 hours
```

Session identity data stores:

```text
Identity issuer
stable Identity sub
email
display name
auth_time
```

No Identity password or bearer token is stored in the BFF session.

## CSRF

`POST /api/auth/csrf` rotates the authenticated Admin CSRF token and returns
the raw value to the same-origin Admin frontend.

Mutating endpoints such as logout require:

```text
X-MW-CSRF header
==
mw_admin_csrf cookie
==
hash stored with session
```

Step 05 uses this on `/api/auth/logout`.

## Local E2E environment

Step 05 keeps using the Step 03 Identity signing key because the persistent
Identity volume already contains signing keys encrypted by it.

The E2E creates:

```text
.env.step5.local
```

containing:

- the existing Identity signing-encryption key;
- a freshly registered confidential BFF client ID/secret;
- a fresh BFF transaction encryption key.

The file is ignored by `.gitignore`.

## E2E

Run:

```powershell
.\scripts\e2e-step-05.ps1
```

It performs the full Browser -> BFF -> Identity -> BFF loop through the
gateway using host routing, then verifies `/auth/me`, CSRF rotation, CSRF
rejection, logout, and session invalidation.

Completion marker:

```text
STEP 05 E2E PASS
```

Step 06 can now build Identity administration behind the authenticated BFF
instead of adding more authentication machinery.
