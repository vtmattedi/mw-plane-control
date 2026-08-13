# Step 03 — Signing Keys, OIDC Discovery, Application Registry

## Scope

Step 03 turns MW Identity into a recognizable OIDC issuer and introduces the
runtime-managed application/client registry.

It does **not** implement `/oauth/authorize` or `/oauth/token` behavior yet.
Those are Step 04.

## OIDC metadata

Public endpoints:

```text
GET /.well-known/openid-configuration
GET /.well-known/jwks.json
```

Canonical issuer:

```text
https://identity.mattediworks.com
```

Discovery advertises the Step 04 target protocol:

```text
Authorization Code
PKCE S256
RS256 ID-token signatures
```

## Signing keys

Identity uses RSA 2048-bit signing keys with:

```text
alg = RS256
use = sig
unique kid
```

The private PKCS#8 key is encrypted before SQLite persistence with AES-256-GCM.

Encryption keyring configuration:

```env
IDENTITY_SIGNING_KEYS=1:<base64url-32-byte-key>
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=1
```

Multiple at-rest key versions can be supplied comma-separated:

```env
IDENTITY_SIGNING_KEYS=1:<old>,2:<new>
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=2
```

The key version in each signing-key row allows old private keys to remain
decryptable during encryption-key rotation.

Identity refuses startup if the configured signing encryption keyring is
missing/invalid.

### Signing-key rotation

Operator command:

```powershell
docker compose exec mw-identity /app/rotate-signing-key
```

Rotation:

1. generates a new RSA key;
2. assigns a fresh `kid`;
3. encrypts the private key with the active at-rest key version;
4. retires the old active signing key;
5. keeps retired public JWKs published.

Keeping retired public verification keys is required so tokens signed just
before rotation remain verifiable for their normal lifetime.

Actual retention/pruning policy is deferred until token lifetime exists in
Step 04.

## Application registry

Initial application record tracks:

```text
name
client_id
client_secret_hash
application_type
authentication_policy
enabled/archive state
exact redirect URIs
exact post-logout redirect URIs
allowed scopes
allowed claims
allowed audiences
timestamps
```

Application types:

```text
confidential
public
```

Confidential applications receive a high-entropy client secret at creation.

The raw secret:

```text
is shown once
is never stored
```

SQLite stores only SHA-256 of the random secret. Because the generated client
secret already has high entropy, a password-style slow KDF is not required.

Client-secret verification uses constant-time comparison.

## Redirect URI policy

Redirect matching is exact.

Outside localhost, authorization redirect URIs must use HTTPS.

This is intentionally stricter than accepting wildcard callback patterns.

## Bootstrap application CLI

Until Identity Admin exists in Step 06, applications are created by operator
CLI:

```powershell
docker compose exec mw-identity `
  /app/create-application `
  --name "MattediWorks Admin" `
  --client-id mw_admin `
  --type confidential `
  --redirect-uri "https://admin.mattediworks.com/api/auth/callback" `
  --scope openid `
  --scope profile `
  --scope email `
  --audience mw-admin
```

The raw client secret is displayed once.

Machine-readable mode:

```text
--json
```

## Local signing environment

Step 03 runtime tests create an ignored local file when needed:

```text
.env.step3.local
```

It contains a random development signing-encryption key so repeated test runs
continue to decrypt the signing keys already persisted in the Identity Docker
volume.

Do not commit this file.

For real deployment, provide the keyring through the deployment secret/env
system.

## E2E

PowerShell:

```powershell
.\scripts\e2e-step-03.ps1
```

It verifies:

1. Identity starts with encrypted signing-key configuration;
2. discovery returns the canonical issuer/endpoints;
3. JWKS publishes an RS256 key with `kid`;
4. application CLI creates a confidential application;
5. one-time client secret is returned;
6. signing-key rotation succeeds;
7. JWKS retains the previous public key after rotation;
8. Step 02 bootstrap/password/session/logout flow still passes.

Completion marker:

```text
STEP 03 E2E PASS
```
