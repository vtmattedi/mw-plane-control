# MW Plane Control Step 10 — MW OAuth Federation

Step 10 replaces the disposable Step 09 resource-server proof with the first real
control-plane downstream integration: MW OAuth.

This is **MW Plane Control Step 10**. It is unrelated to any independently
numbered MW OAuth repository roadmap such as an OAuth-local WebAuthn Step 10.

## Control-plane ownership

The control plane owns:

```text
MW OAuth resource registration in Identity
BFF MW OAuth client
strict BFF route -> downstream route mapping
strict route -> delegated scope mapping
delegated-token acquisition/cache use
browser session + browser/BFF CSRF boundary
```

MW OAuth continues to own project/OAuth business logic and validates the
federated bearer token itself through Identity introspection.

## Resource identity invariant

Step 09 introspection allows a resource server to introspect only a token whose
`aud` exactly equals the introspecting Identity application's `client_id`.

Therefore the Step 10 MW OAuth resource registration is intentionally:

```text
client_id = mw-oauth
audience  = mw-oauth
```

Do **not** register `client_id=mw-oauth-resource` while delegating to
`aud=mw-oauth` unless Identity introspection semantics are intentionally changed.

The current Identity application registry also requires every registered
application to include `openid` in its allowed-scope set. The MW OAuth resource
registration therefore includes `openid` **only as registry metadata**. The BFF
does not request or delegate `openid`; Step 09 explicitly rejects Identity-only
scopes during token exchange. Runtime delegated permissions remain the mapped
`oauth.*` scopes only.

## BFF facade

The browser talks only to the BFF:

```text
/api/oauth/projects
/api/oauth/projects/{id}
/api/oauth/projects/{id}/rotate-key
/api/oauth/projects/{id}/rotate-callback-secret
/api/oauth/projects/{id}/oauth/google
/api/oauth/audit
/api/oauth/audit/{id}
/api/oauth/metrics
/api/oauth/server
/api/oauth/docs
/api/oauth/docs/{version}
```

The gateway strips `/api`, so the BFF handles `/oauth/...`.

There is no generic unrestricted downstream proxy. Every allowed method/path is
mapped to one exact MW OAuth admin path and one exact delegated scope.

## Scope map

```text
GET    projects                         oauth.projects.read
POST   projects                         oauth.projects.write
PATCH  projects/{id}                    oauth.projects.write
POST   projects/{id}/rotate-key         oauth.credentials.rotate
POST   projects/{id}/rotate-callback-secret
                                         oauth.credentials.rotate
GET    projects/{id}/oauth/google       oauth.policy.read
PUT    projects/{id}/oauth/google       oauth.policy.write
GET    audit[/id]                       oauth.audit.read
GET    metrics                          oauth.metrics.read
GET    server                           oauth.server.read
GET    docs[/version]                   oauth.server.read
```

The BFF source Identity application must request these scopes and must allow
`mw-oauth` as an audience. `BFF_DELEGATION_SCOPES` controls the extra scopes
requested at OIDC login.

## Browser security boundary

Mutating facade calls still require the BFF's existing CSRF cookie/header pair.
The BFF does not forward browser cookies, browser `Origin`, or `X-MW-CSRF` to MW
OAuth. It sends only the short-lived delegated bearer token plus safe request
metadata/body.

## Configuration

```env
BFF_OAUTH_BASE_URL=
BFF_OAUTH_HOST=
BFF_OAUTH_AUDIENCE=mw-oauth
BFF_OAUTH_HTTP_TIMEOUT_SECONDS=5
```

An empty `BFF_OAUTH_BASE_URL` disables the integration so the control plane can
be built while the OAuth team finishes hybrid federation support.

`BFF_OAUTH_BASE_URL` is the transport address. `BFF_OAUTH_HOST` is an optional
logical HTTP Host override used when transport and service identity differ.

For example, with MW OAuth running natively on the Windows host while BFF runs
in Docker:

```env
BFF_OAUTH_BASE_URL=http://host.docker.internal:8282
BFF_OAUTH_HOST=oauth.mattediworks.com
```

This preserves MW OAuth Host concealment instead of disabling it.

For that same local topology, native MW OAuth must introspect Identity through
the Windows loopback bridge:

```env
MW_IDENTITY_INTERNAL_BASE_URL=http://127.0.0.1:7100
```

`docker-compose.step10.yml` publishes only:

```text
127.0.0.1:7100 -> mw-identity:7100
```

for the Step 10 local E2E. The base compose remains Docker-private and production
does not require this host publication.

When enabling MW OAuth, set `BFF_DELEGATION_SCOPES` to the OAuth scopes required
by the admin facade.

## Completion

Static/control-plane implementation can land before MW OAuth is ready.

Final Step 10 E2E requires a real MW OAuth hybrid/federated build reachable from
the BFF runtime. It must prove at minimum:

```text
human -> Identity -> BFF
BFF -> token exchange (aud=mw-oauth)
BFF -> MW OAuth with delegated bearer
GET projects succeeds
mutating facade requires BFF CSRF
unmapped facade route returns 404 without reaching MW OAuth
```

Only a real service federation test may print:

```text
STEP 10 E2E PASS
```
