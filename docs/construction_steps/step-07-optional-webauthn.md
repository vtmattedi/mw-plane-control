# Step 07 — Optional WebAuthn at MW Identity

## Goal

Step 07 moves optional passkeys into the true Identity boundary.

The BFF does not implement WebAuthn. The browser performs WebAuthn ceremonies
against MW Identity and a successful passkey login creates the same existing
opaque Identity session + CSRF cookies as password login.

Password authentication remains enabled as the fallback/recovery path.

## Relying-party configuration

Production baseline:

```env
IDENTITY_WEBAUTHN_RP_ID=identity.mattediworks.com
IDENTITY_WEBAUTHN_RP_NAME=MW Identity
IDENTITY_WEBAUTHN_ORIGINS=https://identity.mattediworks.com
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS=1:<base64url-32-byte-key>
IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION=1
IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES=5
```

Generate the encryption key with the local key helper, for example:

```powershell
kg 32 base64url
```

Do not use wildcard WebAuthn origins.

A real browser passkey ceremony must run in a secure context. The local HTTP
Host-header gateway sanity checks can verify API wiring, but Windows Hello /
passkey registration should be tested against the real HTTPS Identity origin
(or a localhost development RP/origin wired specifically for browser testing).

## Credential policy

Registration requests:

```text
resident/discoverable credential = required
userVerification = required
attestation = none
```

Discoverable login also requires user verification.

Each Identity user receives a random stable 64-byte opaque WebAuthn user
handle. The handle is not the email and is not the Identity subject string.
Multiple credentials are supported.

## Persistence and encryption

Step 07 creates:

```text
identity_webauthn_users
identity_webauthn_credentials
identity_webauthn_ceremonies
```

The complete serialized `webauthn.Credential` is encrypted at rest with a
versioned AES-256-GCM keyring. Credential ID and operational metadata remain
available for lookup/update.

WebAuthn `SessionData` is also AES-256-GCM encrypted. Associated data binds
credential material to the Identity user + RP ID + credential ID and binds
ceremony state to the ceremony hash + RP ID + purpose.

## Ceremony tokens

Begin endpoints return an opaque token with prefix:

```text
mwwa_
```

Only SHA-256 of the token is stored.

Ceremonies are short lived (default 5 minutes) and consumed atomically before
verification. A failed verification consumes the ceremony; a retry requires a
new challenge.

## Identity routes

```text
GET    /api/webauthn/credentials
POST   /api/webauthn/register/options
POST   /api/webauthn/register/verify
POST   /api/webauthn/login/options
POST   /api/webauthn/login/verify
DELETE /api/webauthn/credentials/{credential_id}
```

Registration/removal require the existing Identity session plus the current
`X-MW-CSRF` value matching the session CSRF cookie/hash.

The registration/login verify requests also carry:

```text
X-MW-WebAuthn-Ceremony: mwwa_...
```

## Portal

The Identity portal stays deliberately minimal:

- `/login` retains email/password and gains **Sign in with a passkey**;
- `/account` gains a small passkey list/register/remove section;
- browser credential conversion lives in same-origin JS served by Identity.

The login continuation (`return_to`) is still reduced to a safe local relative
path before redirecting after passkey login.

## Audit

Safe events include:

```text
identity.webauthn.registration.started
identity.webauthn.registration.failed
identity.webauthn.credential.registered
identity.webauthn.credential.removed
identity.webauthn.login.completed
identity.webauthn.login.failed
```

Audit metadata never contains raw ceremony tokens, challenges, authenticator
responses, serialized credentials, passwords, session tokens, or CSRF tokens.

## Verification

Run:

```powershell
.\\scripts\\e2e-step-07.ps1
```

The script performs static/API wiring checks and leaves the stack running.
Then complete the required real-browser checkpoint on the configured HTTPS
Identity origin:

```text
password login
-> /account
-> register Windows Hello/passkey
-> logout
-> /login
-> Sign in with a passkey
-> authenticated /account
```

Only after that browser flow succeeds should Step 07 be marked:

```text
STEP 07 E2E PASS
```

Do not start Step 08 before the real passkey flow passes.
