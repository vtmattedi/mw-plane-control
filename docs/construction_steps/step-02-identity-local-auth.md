# Step 02 — Identity Local Account + Bootstrap Admin

## Scope

Step 02 adds the first real authentication boundary to MW Identity.

Implemented:

- Identity users;
- bootstrap superadmin CLI;
- Argon2id password hashes;
- enabled/disabled user state in the data model;
- opaque server-side Identity sessions;
- session token hash-at-rest;
- separate CSRF token hash-at-rest;
- pre-auth login CSRF;
- authenticated logout CSRF;
- plain HTML login/account pages;
- in-memory per-IP login rate limiting baseline;
- authentication/security audit events.

OIDC is **not** implemented yet.

## Bootstrap CLI

Inside the Identity container:

```text
/app/create-superadmin
```

For an interactive terminal:

```powershell
docker compose exec mw-identity `
  /app/create-superadmin `
  --email admin@example.com `
  --display-name "Admin"
```

For automation:

```powershell
"your-password" | docker compose exec -T mw-identity `
  /app/create-superadmin `
  --email admin@example.com `
  --display-name "Admin" `
  --password-stdin
```

The CLI intentionally does not accept a `--password` argument because that
would expose the password in shell/process history.

Minimum initial password length: 12 bytes.

## Password hashing

Passwords use Argon2id with the project's current baseline:

```text
memory      64 MiB
iterations  3
parallelism 2
salt        16 bytes random
key         32 bytes
```

The encoded PHC-style hash is the only stored password representation.

## Identity sessions

Cookie:

```text
mw_identity_session
```

Properties:

```text
HttpOnly
SameSite=Strict
Path=/
host-only
Secure outside development/local/test
```

The raw cookie value is never stored. SQLite stores SHA-256 of the opaque
32-byte-random session token.

Default TTL:

```text
7 days / 168 hours
```

## CSRF

Two stages exist.

### Login CSRF

`GET /login` creates a short-lived random token and places it in:

- a host-only SameSite=Strict cookie;
- a hidden login form field.

`POST /login` requires both values to match.

### Authenticated CSRF

Successful login creates a distinct CSRF token.

The browser receives it in a non-HttpOnly host-only SameSite=Strict cookie.
SQLite stores only its SHA-256 hash attached to the session.

Authenticated mutations require:

1. valid session;
2. form CSRF equals CSRF cookie;
3. raw CSRF hashes to the value stored with the session.

Step 02 uses this for logout.

## Login rate limit

Current baseline is deliberately simple and in-memory:

```text
10 attempts / 5 minutes / client IP
```

It is adequate for the first single-instance implementation and intentionally
not presented as the final distributed abuse-control system.

A successful login resets the IP bucket.

## Audit

Initial event types:

```text
identity.user.created
identity.login.completed
identity.login.failed
identity.logout.completed
```

Unknown-login audit records do not persist the raw attempted email. They use a
short SHA-256 fingerprint prefix instead.

Passwords, session tokens, CSRF tokens, and login bodies are never audit
metadata.

## Runtime E2E

PowerShell:

```powershell
.\scripts\e2e-step-02.ps1
```

The script:

1. rebuilds the stack;
2. waits for Identity health;
3. creates a unique superadmin through the container CLI;
4. loads `/login`;
5. verifies pre-auth CSRF exists;
6. performs password login;
7. verifies the protected account page;
8. verifies incorrect logout CSRF returns 403;
9. performs valid logout;
10. verifies the session no longer accesses `/account`.

Completion marker:

```text
STEP 02 E2E PASS
```

Do not begin Step 03 before this passes.
