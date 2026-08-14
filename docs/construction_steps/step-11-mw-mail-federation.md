# Step 11 — MW Mail federation

Step 11 connects the control plane to the real MW Email Service admin API using
the delegated-token mechanism proven in Step 09 and first integrated with a
real service in Step 10.

## Ownership

The control plane owns:

```text
MW Mail resource registration in Identity
BFF MW Mail client
strict BFF route -> downstream route mapping
strict route -> delegated scope mapping
delegated-token acquisition/cache use
browser session + browser/BFF CSRF boundary
```

MW Mail continues to own email/project business logic and validates the opaque
federated bearer through Identity introspection.

## Resource identity invariant

Under the current Step 09 introspection contract the resource application must
have a client ID equal to the delegated audience:

```text
client_id = mw-mail
audience  = mw-mail
```

The Identity registry currently also requires `openid` in every application's
allowed-scope set. The resource registration includes `openid` only as registry
metadata. The BFF never requests/delegates `openid` to MW Mail.

## Strict BFF facade

The browser talks to:

```text
GET   /api/mail/health
GET   /api/mail/projects
POST  /api/mail/projects
PATCH /api/mail/projects/{id}
GET   /api/mail/projects/{id}/request-domains
PUT   /api/mail/projects/{id}/request-domains
POST  /api/mail/projects/{id}/rotate-key
GET   /api/mail/emails
GET   /api/mail/emails/{id}
GET   /api/mail/emails/by-project
```

The gateway strips `/api`; the BFF handles `/mail/...`. There is no generic
unrestricted proxy. Every accepted method/path maps to one exact downstream
`/admin/api/*` path and one exact delegated scope.

## Scope map

```text
GET   health                              mail.projects.read
GET   projects                            mail.projects.read
POST  projects                            mail.projects.write
PATCH projects/{id}                       mail.projects.write
DELETE projects/{id}                      mail.projects.delete
GET   projects/{id}/request-domains       mail.projects.read
PUT   projects/{id}/request-domains       mail.projects.write
POST  projects/{id}/rotate-key            mail.credentials.rotate
GET   emails                              mail.emails.read
GET   emails/{id}                         mail.emails.read
GET   emails/by-project                   mail.usage.read
GET   docs[/version]                      mail.server.read
```

## Browser/service boundary

Mutating facade calls require the BFF's existing CSRF cookie/header pair. The
BFF does not forward browser cookies, browser `Origin`, or BFF CSRF headers to
MW Mail. It sends the delegated bearer plus safe request metadata/body.

## Transport vs logical Host

```env
BFF_MAIL_BASE_URL=
BFF_MAIL_HOST=
BFF_MAIL_AUDIENCE=mw-mail
BFF_MAIL_HTTP_TIMEOUT_SECONDS=5
```

For the local topology where Mail runs natively on Windows and BFF runs in
Docker:

```env
BFF_MAIL_BASE_URL=http://host.docker.internal:8082
BFF_MAIL_HOST=mailgo.mattediworks.com
```

This preserves Mail's `CHECK_SP_HOST` concealment.

Native MW Mail reaches Identity through the loopback-only Step 11 bridge:

```env
MW_IDENTITY_INTERNAL_BASE_URL=http://127.0.0.1:7100
```

`docker-compose.step11.yml` publishes only `127.0.0.1:7100:7100` for local E2E.
The base compose remains Docker-private.

## Coexistence with MW OAuth

The BFF configuration supports OAuth and Mail simultaneously. A production BFF
Identity application that enables both integrations must allow both downstream
audiences and the union of the required delegated scopes:

```text
audiences: mw-oauth, mw-mail
scopes: oauth.* actually used by the OAuth facade + mail.* actually used by Mail
```

The Step 11 E2E intentionally enables only Mail so failures remain attributable
to the Step 11 boundary; Step 10 already proves the OAuth path independently.

## Mail service handoff

After running the one-time resource registration, configure native MW Mail:

```env
ADMIN_AUTH_MODE=hybrid
MW_IDENTITY_INTERNAL_BASE_URL=http://127.0.0.1:7100
MW_IDENTITY_INTROSPECTION_CLIENT_ID=mw-mail
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=<from .env.step11.mail-resource.local>
MW_IDENTITY_AUDIENCE=mw-mail
MW_IDENTITY_HTTP_TIMEOUT_SECONDS=5
```

Restart the native Mail Go server after changing its environment.

## Unchanged application API

Step 11 does not change normal application email sending:

```http
POST /v1/emails
Authorization: Bearer mw_live_...
Idempotency-Key: ...
```

Identity delegated tokens are only for human administrative operations through
the BFF.

## Completion

Final Step 11 E2E requires the real MW Mail hybrid/federated build. It proves:

```text
human -> Identity -> BFF
BFF -> delegated token (aud=mw-mail)
BFF -> real MW Mail project list/create
BFF browser CSRF boundary
request-domain policy through Mail
credential rotation through Mail
email history/usage reads through Mail
existing `/v1/emails` project API-key authentication remains independent
strict deny-by-default facade
```

MW Mail exposes federated actor attribution in structured logs rather than a
federated admin audit-list route in the current handoff. The E2E prints the
human Identity `sub` so it can be correlated with the Mail log entry.
