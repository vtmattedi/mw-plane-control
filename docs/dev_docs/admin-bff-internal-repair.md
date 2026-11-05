# Repair an existing Admin Web BFF Identity registration

This is a one-time **operator-only** repair for an existing MW Identity application incorrectly registered with `internal=false`. It is intended to address an Admin Web authorization denial for a correctly configured **workforce superadmin**. It is **not** a general-purpose way to make arbitrary applications internal.

The command is available **inside the Identity container**, through Stackport's service/container **Exec / Shell** feature, after deploying an Identity image built with this change.

## Before running

1. Confirm the deployed BFF configuration's **`BFF_OIDC_CLIENT_ID`**. Use the same value below; do not invent a new client ID.
2. Verify that the correct MW Identity database is mounted at `/app/data/identity.db` and the selected Stackport shell belongs to the **mw-identity** service.
3. Take and **verify** an off-container SQLite-safe backup before applying. Be aware that the existing `/app/backup-db` utility calls `ApplyMigrations` before backing up; only use it if you've verified the deployed schema is already current. An explicit database upgrade is outside this hotfix.
4. Confirm the user is an **enabled workforce superadmin**. `internal=true` is necessary for the special superadmin access exemption but not sufficient if the user, client, redirect, scopes or BFF deployment settings are wrong.

## Execute through Stackport

Open the Stackport **Exec / Shell** for the running **mw-identity** container.

Replace `YOUR_EXISTING_CLIENT_ID` with the configured `BFF_OIDC_CLIENT_ID`. The callback below is the expected **Admin Web** callback; use the actual exact registered HTTPS callback if your Admin Web domain is different.

**Dry run (no changes):**

```sh
/app/repair-admin-bff --client-id 'YOUR_EXISTING_CLIENT_ID' \
  --redirect-uri 'https://admin.mattediworks.com/api/auth/callback'
```

Expected outcome: `DRY RUN: ... is eligible for internal=true; no data changed`, or `UNCHANGED: ... is already internal=true`.

**Apply (only after a verified backup and dry run):**

```sh
/app/repair-admin-bff --client-id 'YOUR_EXISTING_CLIENT_ID' \
  --redirect-uri 'https://admin.mattediworks.com/api/auth/callback' --apply
```

Expected outcome: `UPDATED: ... now has internal=true; audited in the Identity database`.

## Safety invariants

The operator tool checks that the selected registration:

- Exists, is enabled, not archived and is a **confidential** client with an existing secret verifier.
- Uses `access_policy=default_deny` (it does **not** relax authorization policy).
- Has the registered exact HTTPS Admin Web callback ending in `/api/auth/callback`, with no query or fragment.
- Has the **`identity.admin`** scope and **`mw-identity-admin`** audience registered.

It changes only **`identity_applications.internal`** and `updated_at_unix`. The update and a `identity.infrastructure.admin_bff_repaired` operator audit event are committed in **one transaction**. If the audit write fails, the promotion rolls back. The command is idempotent; repeated `--apply` calls on an already internal client do not rewrite or rotate anything.

There is **no HTTP endpoint**, no bypass of the normal Identity administrator API, no client-secret rotation, and no application creation. The Identity container's filesystem and database access are the privileged operator boundary.

## After applying

1. Open a new private/incognito browser session to the Admin Web and try the real Identity sign-in again.
2. Verify that the enabled workforce superadmin can obtain the expected admin audience and scopes; confirm ordinary users and public accounts **do not** receive privileged access.
3. Inspect Identity logs and the new audit event, without printing secrets.
4. If login is still denied, investigate the actual client ID, audience, scope, authentication assurance, policy, BFF redirect or session configuration. **Do not set `default_allow` or relax OIDC validation as a workaround.**

This command does **not** redeploy services and does **not** modify production until the operator runs it with `--apply`.

Related: [MW Stackport onboarding](./stackport-onboarding.md).
