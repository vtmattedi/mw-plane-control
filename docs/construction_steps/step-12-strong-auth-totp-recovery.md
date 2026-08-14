# MW Plane Control Step 12 — Strong auth, TOTP, recovery

Step 12 hardens the control-plane Identity boundary after OAuth and Mail federation are stable.

## Assurance hierarchy

```text
primary:   WebAuthn/passkey
fallback:  TOTP (RFC 6238, 6 digits, 30 seconds, +/-1 step)
emergency: manual operator recovery
```

Password hashes remain temporarily for `standard` applications and rollback, but a `strong` application refuses password-only sessions. `recovery` sessions are also insufficient for strong OIDC authorization.

## Application policy

Supported values are now exactly:

```text
standard
strong
```

`standard` preserves existing application choice. `strong` accepts WebAuthn or TOTP for normal authorization, with WebAuthn being the normal path. Explicit WebAuthn step-up uses `acr_values=urn:mw:webauthn` plus `max_age`.

## Sensitive operations

Recent WebAuthn (default five minutes) is required for Identity admin mutations. The BFF also blocks high-risk downstream operations before delegation:

```text
OAuth project API-key rotation
OAuth callback-secret rotation
OAuth Google/callback policy update
Mail project API-key rotation
```

A TOTP session can perform ordinary federated reads/writes but cannot perform these high-risk mutations until a fresh WebAuthn OIDC step-up replaces the BFF session.

## TOTP

The TOTP secret is encrypted at rest with a separate versioned AES-256-GCM keyring. The exact accepted timestep is persisted and cannot be replayed. Setup, confirmation, and disabling require a recent WebAuthn session.

## Manual recovery

Recovery is operator initiated:

```powershell
docker compose run --rm --entrypoint /app/recover-identity mw-identity --email admin@example.com
```

The command revokes Identity sessions and OAuth tokens, marks the account recovery-pending, stores only SHA-256 of the capability, and prints the raw capability once. No email is sent. The URL uses a fragment:

```text
https://identity.mattediworks.com/recover#token=mwrec_...
```

The browser registers a replacement passkey. Completion revokes the old passkeys, removes TOTP, clears recovery-pending, and issues a limited `recovery` session. A fresh passkey sign-in is still required for strong administration.

Cancellation:

```powershell
docker compose run --rm --entrypoint /app/recover-identity mw-identity --cancel --email admin@example.com
```

Previously revoked sessions remain revoked.

## Configuration

```env
IDENTITY_TOTP_SECRET_KEYS=1:<base64url-32-byte-key>
IDENTITY_TOTP_SECRET_ACTIVE_KEY_VERSION=1
IDENTITY_TOTP_ISSUER=MW Identity
IDENTITY_STRONG_AUTH_MAX_AGE_MINUTES=5
IDENTITY_RECOVERY_TTL_MINUTES=10
BFF_STRONG_AUTH_MAX_AGE_MINUTES=5
```

## Browser completion gate

The automated Step 12 E2E proves policy and recovery API boundaries and leaves a localhost browser topology running:

```text
MW Identity  http://localhost:7100
MW BFF       http://localhost:7200
```

It prints a disposable Step 12 user/password. Final hardware acceptance is:

```text
1. Open http://localhost:7100/login and sign in with the printed password.
2. Open /account and register the first passkey. First-passkey bootstrap is the only password-session MFA mutation allowed.
3. Log out, then sign in with the passkey so the Identity session has auth_method=webauthn.
4. On /account, enable TOTP and confirm one authenticator code.

http://localhost:7200/api/auth/callback
5. Open http://localhost:7200/auth/login?return_to=%2F and complete the strong admin login with the passkey.
6. Verify /auth/me reports assurance=high and recent_webauthn=true.
7. Log out and exercise the strong-login TOTP fallback. Verify /auth/me reports assurance=fallback.
8. From the TOTP-backed BFF session, a sensitive OAuth/Mail credential-rotation route must return step_up_required before any downstream call.
9. Follow /auth/step-up and complete WebAuthn. The replacement BFF session must report assurance=high and the sensitive gate must then be satisfied.
10. Start operator recovery with recover-identity; existing Identity/delegated credentials must be revoked. Complete replacement-passkey recovery and then perform a fresh passkey sign-in.
```

Only after this real authenticator flow succeeds should Step 12 be recorded as `STEP 12 E2E PASS`.
