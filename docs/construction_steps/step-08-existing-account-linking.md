# Step 08 — Existing account linking

## Goal

Step 08 proves the reusable account-linking contract for applications that
already own local users, such as WOI.

The relying application remains the owner of its local/domain user. MW
Identity contributes an immutable OIDC subject (`sub`). The application owns
the mapping between the two.

```text
local user 791
    <->
issuer = https://identity.mattediworks.com
sub    = usr_...
```

Identity assignments and application-local links are deliberately separate:

```text
Identity assignment
    May usr_... use WOI?

WOI external identity mapping
    Which WOI local user does usr_... represent?
```

The BFF does not own ordinary application links.

## Production data model

A relying app should keep a table equivalent to:

```sql
CREATE TABLE external_identities (
    id TEXT PRIMARY KEY,
    local_user_id TEXT NOT NULL,
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    linked_at TIMESTAMP NOT NULL,
    last_login_at TIMESTAMP NULL,
    UNIQUE (issuer, subject),
    UNIQUE (local_user_id, issuer)
);
```

`UNIQUE (issuer, subject)` prevents one Identity subject from being moved to a
second local account. The optional second uniqueness constraint enforces one
subject per issuer for a local account.

Email is profile data, never the permanent foreign key.

## Link from an existing authenticated account

Preferred flow:

```text
existing local app session
-> Link MattediWorks Identity
-> create short-lived OIDC link transaction
-> Authorization Code + PKCE S256 + state + nonce
-> MW Identity authentication
-> validate signature/issuer/audience/expiry/nonce
-> obtain stable sub
-> verify the SAME local app session still owns the link transaction
-> insert (local_user_id, issuer, sub)
```

This proves control of both accounts.

The link transaction should be single-use, short-lived, and bound to the
local account/session that initiated it.

## First Sign in with MW Identity when no link exists

```text
OIDC succeeds
-> validated issuer + sub
-> no external_identities row
-> DO NOT search email and auto-link
-> create short-lived pending-link capability
-> require existing local authentication
-> only then create mapping
```

Even when the OIDC email exactly equals a local account email, email equality
alone is insufficient proof of account ownership.

A signup-capable application may offer new-account creation here according to
its own signup policy. That is outside this proof harness.

## Safe unlink

Self-service unlink must require fresh local/application authentication and
must verify that an alternate login or recovery path remains available.

The Step 08 proof harness requires the local password again and revokes all
of its local sessions after unlink. Identity-only applications may forbid
self-service unlink completely.

## OIDC validation

A relying application must validate at least:

```text
state
PKCE verifier
ID-token signature against JWKS
iss exact match
aud contains this client_id
exp / iat
nonce
sub present
exact redirect URI
```

Do not log authorization codes, raw state, raw pending-link capabilities,
client secrets, or session tokens.

## Reference RP

`testapps/account-link-rp` is a disposable integration harness. It is not a
production service and must not be deployed.

It intentionally uses two local test credentials injected through environment
variables so the proof does not prescribe how WOI or another application
stores/verifies its existing passwords.

The harness implements:

```text
POST /local/login
POST /local/logout
GET  /me
GET  /link/start
GET  /auth/mw/start
GET  /oidc/callback
GET  /link-required
POST /link-existing
POST /unlink
```

Its OIDC client validates the real MW Identity RS256 ID token using the real
JWKS endpoint. Transactions, local sessions, and pending-link capabilities are
held only in memory and are stored/compared by hashes where applicable. The
actual `external_identities` behavior is exercised in memory; production apps
must persist that mapping in their own database.

## E2E checkpoint

Run:

```powershell
.\scripts\e2e-step-08.ps1
```

The checkpoint proves:

```text
existing local login
-> link MW Identity
-> logout local app
-> sign in through linked Identity

same subject -> second local user
-> rejected

fresh local auth
-> safe unlink + app-session revocation

Sign in with MW Identity after unlink
-> same email exists locally
-> NO auto-link
-> existing local authentication required
-> mapping recreated

remove Identity assignment
-> authorization denied / no code issued

restore assignment + disable application
-> authorization denied / no code issued
```

Completion marker:

```text
STEP 08 E2E PASS
```

Do not begin Step 09 before this checkpoint passes.
