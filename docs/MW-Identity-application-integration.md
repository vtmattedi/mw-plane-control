# Integrating an application with MW Identity

MW Identity is the authoritative identity provider. A consuming application keeps its own business profile and links it to the immutable Identity `sub`; it must not use email as its permanent key.

## 1. Register the application

Create an Identity application with:

- a stable, unique `client_id`;
- type `confidential` for a server-side application or `public` when no secret can be protected;
- exact HTTPS redirect URIs;
- exact post-logout redirect URIs when logout returns to the application;
- scopes `openid profile email` as needed;
- an HTTPS whole-MW-account deletion callback (`delete_callback_uri`), which is **required** (see below);
- public signup enabled only when the application accepts self-service public accounts.

A confidential client secret is displayed once on creation and rotation. Store it in the application's secret manager.

### The deletion contract

Only vetted applications use MW Identity, and implementing deletion is a condition of that vetting. Every application that signs people in with MW Identity **must** implement the `identity.user.deleted` callback described in section 6, and agrees to it in its terms of use or another written agreement with MattediWorks. Identity enforces the technical half: it refuses to create or edit an application without `delete_callback_uri` (`400 delete_callback_required`).

The only exception is an **internal** application (`internal: true`): workforce tooling that runs behind the MW BFF and keeps no user data of its own, such as the admin console's BFF client and the resource servers it calls. Internal applications cannot enable public signup. Removing one from an account still revokes its access immediately; there is nothing for it to delete.

Applications registered before the callback became mandatory keep working. The admin API marks them with `needs_delete_callback: true`, and their next edit must add the callback (or mark them internal). Enabling, disabling and archiving them stays available. With the CLI, pass `--delete-callback-uri` or `--internal`.

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

## 4. Public enrollment, eligibility and consent

Public signup permits a new OR existing MW identity to enroll in an eligible application. It never grants an administrative role. For a non-internal, non-admin application, an unassigned user is eligible when access_policy=default_allow OR allow_public_signup=true. An explicit deny still wins, subject to the existing global-superadmin policy exception. Disabled/archived applications, disabled identities, private applications and workforce/admin audience boundaries remain enforced.

Registration creates a public MW identity; it no longer creates an administrative application assignment. Stand-alone registration remains controlled by IDENTITY_PUBLIC_SIGNUP_ENABLED. A local app profile is created only through that app's onboarding. Email verification does not currently block signup; treat email_verified=false as untrusted mailbox ownership.

Non-internal applications require explicit consent to the requested identity scopes. Identity displays the app and scopes before issuing a code when consent is absent, revoked or expanded. Cancel grants nothing. Consent is durable and independent of assignment rows and access-token expiry. Existing identities can consent without creating another MW account.

Explicitly internal clients are trusted workforce tooling: their initial consent is bootstrapped without a public consent prompt. This exception never admits public identities to the admin audience. Explicit disconnection still requires consent on return. Marking a third-party client internal to bypass consent or callbacks is prohibited.

The provider enforces max_age for ordinary authentication as well as WebAuthn. max_age=0 requires a new authentication session; ID tokens include auth_time. Apps requiring recent authentication must validate that claim, bind the subject to the intended action, and obtain a separate final confirmation before deleting data.

## 5. Connected applications and disconnect

Active consent defines the connected-app list. Administrator assignments remain authorization policy, not user-owned connections. Token expiry does not remove consent. Known-app history survives disconnect for whole-account notification discovery.

**Disconnect MW sign-in** revokes that application's consent, authorization codes and access tokens, including its delegated tokens. It sends NO deletion callback, preserves administrator assignments, and does not delete the application account, data or memberships. It does not log the user out of the application's own sessions. No callback availability or deletion acknowledgement is needed.

A subsequent sign-in requires consent again. The application should reopen the same issuer/sub-mapped profile when it still exists. Disconnect does not make an app account new. Apps must never link by matching email.

To delete an application account, use the application's authenticated deletion workflow. The app invalidates its own sessions/credentials and deletes or anonymizes its data. A later permitted sign-in may create a new local profile through explicit onboarding. Provider consent may remain until disconnected separately; this first release does not provide an app-owned provider-revocation API.

## 6. Whole-MW-account deletion callbacks

Deleting the WHOLE MW account remains separate from disconnect. Identity disables the identity and revokes its sessions/codes/tokens, then notifies known applications, including previously disconnected ones. Unknown associations that already disappeared before consent history was introduced cannot be reconstructed. Do not claim this list proves where all historical copies of a person's data exist.

```json
{
  "id": "evt_...",
  "type": "identity.user.deleted",
  "sub": "usr_bM8...",
  "application_id": "app_...",
  "client_id": "woi-production",
  "reason": "account_deleted",
  "requested_at": "2026-10-27T12:00:00Z"
}
```

Requests use Authorization: Bearer <signed lifecycle JWT> and X-MW-Identity-Event-ID. Verify RS256/JWKS, exact issuer and client audience, expiration, event type, subject, and the same event ID in JWT jti, header and body. Require the signed JWT reason to equal the body reason before destructive processing. Only signed reason=account_deleted authorizes this destructive action in the revised contract. A valid legacy application_disconnected event must be acknowledged as ignored, without deleting data or setting a sign-in cutoff. Unknown/missing reasons fail explicitly.

WOI returns success only after required local cleanup completes; failures remain retryable. Identity's existing routine makes three delivery attempts: immediately, after 250 ms and a further 750 ms. Redirects are rejected. Failed whole-account deletion leaves the central identity disabled for an administrator to retry. The current provider still treats any 2xx as acknowledgement; it has no separate asynchronous completion protocol. Do not return 202 from a receiver that expects Identity to track later completion.

Identity cannot inspect application databases or guarantee erasure of backups/exports. Central revocation does not instantly end an unreachable application's independent sessions. Existing whole-account callback operation IDs/retry recovery have known limitations; this release changes per-app disconnect, not that recovery protocol. Internal/callback-less legacy applications remain reported as skipped, not verified data erasure.

## 7. Upgrade

See [federated-signin-rollout.md](dev_docs/federated-signin-rollout.md) for migrations, ordering, internal-client handling and incompatible rollback. Deploy consumer ignore-disconnect safeguards before enabling the revised provider behavior. This is a contract change: do not run an old destructive receiver during the transition.
