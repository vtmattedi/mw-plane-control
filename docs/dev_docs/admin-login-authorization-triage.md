# Admin Web OIDC authorization-denial triage (ENG-47)

**Scope:** `admin.mattediworks.com` -> MW BFF -> `identity.mattediworks.com`. This is an operator runbook, **not** authorization-policy migration instructions.

## Identify the failure stage

`{"error":{"code":"identity_authorization_failed"}}` was the BFF's generic response when Identity redirected back with an OAuth `error` query parameter. This indicates **no authorization code was received**, before the BFF could create an Admin Web session. The error alone does *not* identify the underlying cause or prove that the provider is defective.

The BFF now consumes/validates the one-time `state` on error and emits bounded non-secret categories:

| BFF callback response | Meaning | Recommended check |
| --- | --- | --- |
| `identity_application_access_denied` (403) | Identity explicitly denied this account under application policy | Check BFF app classification, access policy, assignment and account type |
| `identity_consent_declined` (403) | Consent page was declined | Retry consent if intended; do **not** grant consent silently |
| `identity_access_denied` (403) | Access denied, reason not safely classified | Check Identity logs and registered client policy |
| `identity_authorization_request_rejected` (400) | Identity rejected the request (e.g. unregistered scope/redirect) | Compare BFF requested scopes with registered app |
| `identity_provider_unavailable` (502) | Identity reported a provider failure | Check provider health/logs |
| `invalid_or_expired_state` (400) | BFF cannot validate a pending login transaction | Retry clean login; check BFF persistence and transaction TTL |
| `identity_authorization_failed` (400) | Unknown provider failure category | Inspect provider error safely in operator logs |

**Do not** record/replay raw `code`, `state`, cookies, client secrets, or arbitrary `error_description` in issue trackers. The callback response does not display untrusted provider descriptions.

## Likely source of denial after Federation v1.0 (verify; do not assume)

The 2026-11-04 clean migration intentionally removed ordinary-application superadmin bypasses. In `mw-identity/internal/oidc/admin_access.go`, privileged bypass is still available **only** if all are true:

1. Identity user is an enabled **workforce** superadmin (not a public identity).
2. BFF application is explicitly classified `internal=true`, is enabled, and is not archived.
3. It permits `identity.admin` scope and `mw-identity-admin` audience.
4. BFF actually requests `identity.admin`.

Otherwise, the ordinary Public/Private policy applies: explicit deny always wins; for `default_deny`, an explicit allow assignment is required. Do not turn an administrative application Public to repair an access-denied symptom.

`mw-identity/cmd/create-application` defaults to `--access-policy default_deny`, and `--internal` is **opt-in**. Previously provisioned Admin BFF clients should be checked against the stricter contract, especially if registration predates the migration.

## Non-secret diagnostic checks (read-only)

Use the deployed **existing** BFF client ID in a private operator environment. Never paste client secrets or live database rows into Linear/GitHub. Take an appropriate database snapshot before any maintenance.

At the Identity SQLite store, use parameterized read-only queries (replace the parameter through your local DB client, not by building SQL from an HTTP request):

```sql
SELECT client_id, application_type, internal, access_policy, enabled,
       archived_at_unix IS NULL AS not_archived
FROM identity_applications
WHERE client_id = ?;

SELECT scope FROM identity_application_scopes
WHERE application_id = ? ORDER BY scope;
SELECT audience FROM identity_application_audiences
WHERE application_id = ? ORDER BY audience;
SELECT redirect_uri FROM identity_application_redirect_uris
WHERE application_id = ? ORDER BY redirect_uri;

SELECT account_type, is_superadmin, enabled,
       deletion_requested_at_unix IS NULL AS not_deleting
FROM identity_users WHERE id = ?;

SELECT access_effect FROM identity_application_assignments
WHERE application_id = ? AND user_id = ?;
```

Interpret with the actual registered application/user IDs and the BFF's **non-secret** `BFF_OIDC_CLIENT_ID`, `BFF_PUBLIC_BASE_URL`, `BFF_IDENTITY_ISSUER`, and `BFF_DELEGATION_SCOPES`. Ensure `https://admin.mattediworks.com/api/auth/callback` matches exactly and that registered scopes include *all* BFF-requested scopes, including `identity.admin`.

If the failure category is `identity_authorization_request_rejected`, check whether the deployed BFF requests scopes introduced after its admin app was registered. Identity rejects unregistered scopes; do not suppress or silently strip required privileges to make the sign-in appear successful.

## Recovery decision

- **Client is correct but identity is not eligible:** refuse login; grant only reviewed individual assignment/role where appropriate. Do not promote a public account to workforce or superadmin automatically.
- **Legacy Admin BFF registration is incorrectly non-internal:** use the existing audited infrastructure-provisioning procedure and preserve the original client ID, secret, grants, and consent data. No blanket `internal=true` SQL update is authorized by this document.
- **Client scopes/redirect mismatch:** review and update registration through approved administrative tools, and ensure BFF credentials match the existing client. Do not create a second client as a quick workaround.
- **Bad/expired state:** retry after clearing stale browser auth flow; review BFF transaction storage/TTL and proxy behavior.
- **Unknown:** collect sanitized BFF/Identity log categories and compare the deployed versions with the codebase. If the root cause remains unproven, do not present the code change as a verified production outage fix.

## Validation

The source regression tests cover bounded provider-error classification, state binding, one-time consumption, replay and unknown error descriptions. Full integration still requires a disposable, correctly registered Admin BFF client plus negative tests for private/denied/public users and a real browser sign-in. No live production database or account was accessed by this investigation.

Related: [ENG-47](https://linear.app/mattediworks/issue/ENG-47/admin-web-login-fails-with-identity-authorization-failed), [federation v1 migration](federation-v1-clean-migration.md), [Stackport onboarding](stackport-onboarding.md).
