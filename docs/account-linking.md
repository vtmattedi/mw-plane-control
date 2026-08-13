# Existing Account Linking to MW Identity

## Requirement

Existing MattediWorks applications may already have users.

When MW Identity is introduced, those users must be able to link their
existing app account without losing:

- history;
- profile/business data;
- permissions;
- subscriptions;
- local user ID;
- other application state.

The application remains owner of the local domain user.

MW Identity supplies a stable external subject identifier: `sub`.

## Core model

A relying app should keep a mapping similar to:

```text
external_identities
  id
  local_user_id
  issuer
  subject
  linked_at
  last_login_at
```

For MW Identity:

```text
issuer  = https://identity.mattediworks.com
subject = <OIDC sub>
```

At minimum:

```text
UNIQUE (issuer, subject)
```

Do not use email as the permanent foreign key.

## Preferred linking flow

Safest flow: the user is already logged into the application.

```text
existing app session
  |
  | "Link MattediWorks Identity"
  v
app creates short-lived linking transaction
  |
  v
redirect to MW Identity
  |
  v
user authenticates
  |
  v
app callback
  |
  + validate state
  + validate PKCE
  + validate issuer/audience/signature
  + obtain stable sub
  |
  v
link sub to CURRENT authenticated local user
```

This proves control of both the existing local account and MW Identity.

## First "Sign in with MattediWorks" when no link exists

If MW Identity login succeeds but no app mapping exists:

```text
sub = usr_abc
```

do **not** automatically link only because Identity email matches a local
account email.

Instead:

```text
Identity login succeeds
  |
  v
no link exists
  |
  v
"An existing account may already exist.
 Sign in to your existing account to link it."
  |
  v
user authenticates existing app account
  |
  v
link Identity sub
```

If no local account exists, the app may create one according to its signup
policy.

## Why email auto-linking is avoided

Email equality alone can create account-takeover edge cases involving:

- stale or reassigned addresses;
- inconsistent normalization;
- legacy weakly verified email;
- different app ownership rules;
- future providers with different guarantees.

Use immutable `sub` as the actual link.

## Identity-side app assignments

Identity may know:

```text
usr_abc -> WOI -> allowed
```

That answers:

```text
May this Identity use WOI?
```

It does not replace the app mapping:

```text
WOI local user 791 -> Identity usr_abc
```

That mapping belongs to WOI.

## Linking initiated from Identity

A future Identity portal may show connected applications.

A "Connect WOI" action should redirect to an app-owned link endpoint. Identity
must not invent the app's local user mapping.

## Unlinking

Before unlinking:

- require recent authentication;
- ensure another login/recovery path exists where required;
- prevent lockout;
- revoke relevant sessions if policy requires;
- audit the event.

Identity-only apps may forbid self-service unlink.

## Admin migration

MattediWorks Admin is expected to become Identity-native.

Existing MW OAuth/Mail local admin accounts are migration compatibility
accounts.

Migration can be:

```text
local admin
-> map/link to Identity subject
-> hybrid period
-> federated production
```

Per service, authorization may eventually rely directly on Identity claims
rather than a permanent local admin row.

## BFF involvement

The Admin BFF does not own ordinary application account links.

For WOI:

```text
WOI <-> MW Identity
```

is the relevant relationship.

The BFF is only involved in the shared MattediWorks Admin application.
