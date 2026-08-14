# Step 09 — Delegated downstream service tokens

Step 09 adds a narrow on-behalf-of path from MW BFF to downstream MattediWorks
services without forwarding the long-lived Identity-admin bearer token.

## Flow

```text
human admin session
  -> BFF decrypts its Identity subject token in memory
  -> OAuth token exchange at MW Identity
  -> short-lived opaque token
       sub = human Identity subject
       aud = one exact downstream service
       scope = requested subset only
       auth_time / amr retained
  -> BFF encrypted cache bound to its admin session
  -> downstream resource server
  -> authenticated Identity introspection
```

The exchange endpoint uses the OAuth token-exchange grant identifier and
access-token token-type identifiers. The implementation intentionally supports
only the subset needed by the control plane: one subject access token, one
exact audience, and narrowed scopes.

## Security invariants

- The requesting confidential client must authenticate.
- The subject token must have been issued to that same client.
- The requested audience must be explicitly registered on that client.
- `mw-identity-admin` cannot be minted through delegation.
- Delegated scopes must be present on both the subject token and client.
- OIDC/Identity-only scopes are not delegatable.
- Delegated lifetime is short and never exceeds the subject token lifetime.
- Raw subject/delegated tokens are never logged.
- The BFF cache encrypts delegated tokens with its existing versioned keyring.
- Cache rows are tied to the BFF admin session and cascade on session deletion.
- Resource-server introspection only reveals a token when `aud` equals the
  authenticating resource server's client ID.

## Test resource server

`testapps/delegation-rs` is a disposable Step 09 proof resource server. It is
started only through `docker-compose.step09.yml` and is not part of the normal
production topology.

The development-only BFF probe route is enabled only when the Step 09 test
resource URL/audience/scope environment variables are provided, and is
suppressed when `APP_ENV=production`.

## Completion

```text
Admin user
-> BFF session
-> token exchange
-> aud = Step 09 test resource server
-> mock RS introspection
-> same human sub observed at RS
-> auth_time/amr retained
-> second call uses encrypted BFF cache
```

Only then record `STEP 09 E2E PASS`.
