# Integrating an application with MW Identity

MW Identity is the authoritative identity provider. A consuming application keeps its own business profile and links it to the immutable Identity `sub`; it must not use email as its permanent key.

## 1. Register the application

Create an Identity application with:

- a stable, unique `client_id`;
- type `confidential` for a server-side application or `public` when no secret can be protected;
- exact HTTPS redirect URIs;
- exact post-logout redirect URIs when logout returns to the application;
- scopes `openid profile email` as needed;
- an optional HTTPS account deletion callback;
- public signup enabled only when the application accepts self-service public accounts.

A confidential client secret is displayed once on creation and rotation. Store it in the application's secret manager.

## 2. Use Authorization Code with PKCE

Discover endpoints from:

```text
https://identity.mattediworks.com/.well-known/openid-configuration
```

Start authorization with `response_type=code`, an exact registered `redirect_uri`, a random `state`, a random `nonce`, and an S256 PKCE challenge. On callback, verify `state`, exchange the code at `/oauth/token`, and validate the ID token signature through the published JWKS.

Validate at least `iss`, `aud`, `exp`, `nonce`, and the RS256 signature. A code and PKCE verifier are one-time values.

To choose the portal language, send the standard `ui_locales` parameter (for example `ui_locales=en`). Without it, Identity follows the browser's preferred language and falls back to Portuguese. The sign-in and sign-up pages, and the hand-off back to your application, all keep that language.

## 3. Identity payload

With `openid profile email`, the ID token and `/oauth/userinfo` expose the following user identity data:

```json
{
  "sub": "usr_bM8...",
  "email": "person@example.com",
  "email_verified": false,
  "name": "Public User",
  "preferred_username": "public.user",
  "mw_account_type": "public"
}
```

The ID token additionally contains protocol claims such as `iss`, `aud`, `exp`, `iat`, `auth_time`, `amr`, and the requested `nonce`.

- `sub` is the immutable identifier and the correct foreign key for an application account.
- `email` can change and is not proof of ownership when `email_verified` is false.
- `preferred_username` is present for public accounts with a username and may change under future account policy.
- `mw_account_type` is `public` or `workforce`.
- Password hashes, application roles, and application-domain data are never included.

A typical local mapping is:

```text
application_users
  id                    application-local primary key
  identity_issuer       https://identity.mattediworks.com
  identity_subject      usr_bM8...
  UNIQUE(identity_issuer, identity_subject)
```

## 4. Public enrollment

There are two ways to create a `public` account. Neither can create a `workforce` account or a superadmin.

- **From an application.** When `allow_public_signup` is enabled, the Identity login screen offers account creation while processing an authorization request for that application. Registration creates an enabled `public` user and a `user` assignment to the requesting application in one transaction, then continues the authorization.
- **Stand-alone.** When `IDENTITY_PUBLIC_SIGNUP_ENABLED` is true (the default), `/register` and `/register/en` create an enabled `public` user with no assignment and sign them in to `/account`. A plain visit to the login page links to it; a login that returns to an application or asks for step-up does not. The account can then use any application whose access policy admits it (`default_allow`, or an explicit assignment).

Both forms share a per-IP registration rate limit that is separate from the login limit, and carry a hidden honeypot field; a submission that fills it is rejected.

Email verification does not currently block registration or login. Applications must treat an unverified email as user-supplied data and use the `email_verified` claim when a trusted mailbox is required. The account page displays the verification state.

Public identities cannot authorize for the `mw-identity-admin` audience. The Admin BFF must continue to require the admin audience, admin scopes, assignment role, and a `workforce` account.

## 5. Connected applications and deletion

The Identity account page derives connected applications from assignments and issued access tokens. Applications with local user data should configure `delete_callback_uri`.

The same `identity.user.deleted` event is sent in two situations. In both, the application must treat the subject as gone and delete or anonymize its local data:

- **Account deletion** (`reason: account_deleted`). The user or an administrator deletes the account. Identity first disables the user and revokes sessions, authorization codes and access tokens, then sends the event to every configured callback.
- **Application removal** (`reason: application_disconnected`). The user removes one application from the account page. Identity deletes that application's authorization codes and access tokens for the user, then sends the event to that application only. The Identity account and every other application are unaffected, and the user may sign in to the application again later if its access policy allows it.

```json
{
  "id": "evt_...",
  "type": "identity.user.deleted",
  "sub": "usr_bM8...",
  "application_id": "app_...",
  "client_id": "woi-production",
  "reason": "account_deleted",
  "requested_at": "2026-08-15T02:30:00Z"
}
```

`reason` is informational: process both values the same way. Applications that predate it keep working because they validate `type`, not `reason`.

The request includes:

```text
Authorization: Bearer <signed lifecycle JWT>
X-MW-Identity-Event-ID: evt_...
Content-Type: application/json
```

Validate the lifecycle JWT with Identity JWKS. Require `iss` to match the configured issuer, `aud` to equal your `client_id`, `event` to equal `identity.user.deleted`, `sub` to equal the JSON subject, and `jti` to equal the event header/body ID.

Return any 2xx status only after the event has been durably accepted. Processing must be idempotent by event ID and subject. Identity makes three total attempts: immediately, after 250 ms, and after a further 750 ms. Redirect responses are not followed.

The application should immediately disable its local account and sessions, then delete or anonymize application data according to its retention obligations.

If delivery fails:

- for account deletion, Identity keeps the central account disabled and deletion-pending so an administrator can trigger the same routine again from MW Control;
- for application removal, the application's access stays revoked but the user's assignment is kept, so the user can retry from the account page. A `deny` assignment set by an administrator is never removed by the user.

Identity cannot verify that an application actually deleted its data; a 2xx response is the application's commitment that it accepted the request. Applications without `delete_callback_uri` are skipped, and the account page tells the user so.
