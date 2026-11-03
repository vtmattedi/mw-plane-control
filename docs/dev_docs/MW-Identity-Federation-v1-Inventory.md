# Federation v1 baseline dependency inventory

Baseline: `3bf0ca4f53139568f711160a7c269843557708d7` (2026-11-03). Generated from tracked Go, TypeScript/TSX, JavaScript, SQL, PowerShell, HTML and Markdown plus Compose/config sources. Environment files, keys, lockfiles and generated assets are excluded. Line numbers refer to that baseline, not subsequent commits. Groups include definitions, reads, writes and test/doc references. `internal/` import paths are not infrastructure classification.

This is a source inventory, not an audit of live client registrations. Regenerate/search before each dependent increment.

## App authentication policy

| Baseline file | Matching lines |
| --- | --- |
| `docs/MW-Control-Plane-v1.0-documentation.md` | 182 |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 333 |
| `docs/MW-Identity-Implementation-guide.md` | 224 |
| `docs/construction_steps/step-03-signing-discovery-app-registry.md` | 97 |
| `docs/construction_steps/step-06-identity-administration.md` | 175 |
| `docs/dev_docs/mw-identity.md` | 103 |
| `docs/dev_docs/stackport-onboarding.md` | 92 |
| `mw-identity/cmd/create-application/main.go` | 39, 166 |
| `mw-identity/internal/applications/model.go` | 38, 63 |
| `mw-identity/internal/applications/service.go` | 27, 65, 156, 183 |
| `mw-identity/internal/applications/service_test.go` | 22 |
| `mw-identity/internal/httpapi/consent_test.go` | 24 |
| `mw-identity/internal/httpapi/identity_admin.go` | 231, 255 |
| `mw-identity/internal/httpapi/oauth.go` | 98 |
| `mw-identity/internal/httpapi/registration_test.go` | 174, 210 |
| `mw-identity/internal/identityadmin/model.go` | 36 |
| `mw-identity/internal/identityadmin/service.go` | 29, 166, 653, 655, 662, 663, 665 |
| `mw-identity/internal/store/sqlite/account_lifecycle_test.go` | 54 |
| `mw-identity/internal/store/sqlite/applications.go` | 45, 60, 176, 194 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 46, 109, 164, 189, 247 |
| `mw-identity/internal/store/sqlite/identity_admin.go` | 123, 132 |
| `mw-identity/migrations/004_identity_applications.sql` | 8 |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx` | 37, 56, 130, 143, 408, 411 |
| `mw-web-admin/src/pages/Control/ResourcePage.tsx` | 304 |
| `mw-web-admin/src/pages/Control/pages.tsx` | 62 |
| `scripts/e2e-step-08.ps1` | 284 |
| `scripts/e2e-step-12.ps1` | 174, 199 |
| `scripts/sanity-step-06.ps1` | 260, 319 |

## Per-app signup

| Baseline file | Matching lines |
| --- | --- |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 77, 336 |
| `docs/MW-Identity-application-integration.md` | 76 |
| `docs/dev_docs/mw-identity.md` | 104 |
| `mw-identity/cmd/create-application/main.go` | 54, 171 |
| `mw-identity/internal/applications/model.go` | 45, 69 |
| `mw-identity/internal/applications/service.go` | 32, 113, 162, 189 |
| `mw-identity/internal/applications/service_test.go` | 212 |
| `mw-identity/internal/auth/registration.go` | 36 |
| `mw-identity/internal/auth/store.go` | 101 |
| `mw-identity/internal/config/config.go` | 38 |
| `mw-identity/internal/httpapi/consent_test.go` | 24 |
| `mw-identity/internal/httpapi/identity_admin.go` | 236, 260 |
| `mw-identity/internal/httpapi/registration_test.go` | 175, 211 |
| `mw-identity/internal/identityadmin/model.go` | 41 |
| `mw-identity/internal/identityadmin/service.go` | 34, 171, 702 |
| `mw-identity/internal/oidc/admin_access.go` | 148 |
| `mw-identity/internal/store/sqlite/applications.go` | 52, 66, 183, 218 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 48, 90, 91, 287 |
| `mw-identity/internal/store/sqlite/consent.go` | 58 |
| `mw-identity/internal/store/sqlite/consent_test.go` | 132, 151 |
| `mw-identity/internal/store/sqlite/identity_admin.go` | 126, 135 |
| `mw-identity/internal/store/sqlite/oidc.go` | 68, 134, 205 |
| `mw-identity/internal/store/sqlite/registration.go` | 17, 28 |
| `mw-identity/migrations/010_account_lifecycle.sql` | 5, 6 |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx` | 44, 61, 135, 148, 457, 460, 491, 494 |
| `mw-web-admin/src/pages/Control/ResourcePage.tsx` | 311 |

## Infrastructure classification

| Baseline file | Matching lines |
| --- | --- |
| `README.md` | 129 |
| `docker-compose-dev.yml` | 119, 123, 134 |
| `docs/MW-Bff-implementation-guide.md` | 11, 17, 38, 89, 163, 386, 429, 551, 554, 570, 694, 736 |
| `docs/MW-Control-Plane-v1.0-documentation.md` | 107 |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 81, 338 |
| `docs/MW-Identity-Implementation-guide.md` | 131, 133 |
| `docs/MW-Identity-application-integration.md` | 23, 25, 76, 80, 82, 120 |
| `docs/construction_steps/step-03-signing-discovery-app-registry.md` | 147 |
| `docs/construction_steps/step-04-authorization-code-pkce.md` | 81 |
| `docs/construction_steps/step-05-bff-oidc-login.md` | 41 |
| `docs/construction_steps/step-10-mw-oauth-federation.md` | 118 |
| `docs/construction_steps/step-11-mw-mail-federation.md` | 94 |
| `docs/dev_docs/federated-signin-rollout.md` | 8, 21, 22, 25 |
| `docs/dev_docs/federated-signin-verification.md` | 17, 49, 128 |
| `docs/dev_docs/federation.md` | 20 |
| `docs/dev_docs/implement-federation.md` | 388, 959, 1385 |
| `docs/dev_docs/mw-bff.md` | 56 |
| `docs/dev_docs/mw-identity.md` | 54, 105, 106, 111, 112, 113, 292 |
| `docs/dev_docs/mw-web-admin.md` | 75 |
| `docs/dev_docs/project.md` | 166 |
| `docs/dev_docs/stackport-onboarding.md` | 92 |
| `mw-identity/cmd/create-application/main.go` | 51, 58, 59, 61, 172, 186, 209 |
| `mw-identity/internal/applications/model.go` | 13, 17, 19, 20, 46, 70 |
| `mw-identity/internal/applications/service.go` | 33, 113, 163, 190 |
| `mw-identity/internal/applications/service_test.go` | 211, 212, 223, 224 |
| `mw-identity/internal/httpapi/identity_admin.go` | 237, 261 |
| `mw-identity/internal/identityadmin/model.go` | 42 |
| `mw-identity/internal/identityadmin/service.go` | 35, 172, 190, 301, 702 |
| `mw-identity/internal/oidc/admin_access.go` | 148 |
| `mw-identity/internal/oidc/consent.go` | 18 |
| `mw-identity/internal/store/sqlite/applications.go` | 53, 67, 165, 184, 202, 219, 220 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 255, 256, 258, 259, 260, 283, 284, 285, 289 |
| `mw-identity/internal/store/sqlite/consent.go` | 58 |
| `mw-identity/internal/store/sqlite/consent_test.go` | 46, 47, 67 |
| `mw-identity/internal/store/sqlite/identity_admin.go` | 127, 136 |
| `mw-identity/internal/store/sqlite/oidc.go` | 68, 134, 205 |
| `mw-identity/migrations/012_application_internal.sql` | 1, 5, 6 |
| `mw-identity/migrations/013_application_consent.sql` | 29, 31, 33, 34, 36, 41, 45 |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx` | 45, 62, 136, 149, 456, 458, 463, 466, 472, 487 |
| `mw-web-admin/src/pages/Control/ResourcePage.tsx` | 97, 198, 312, 674, 676, 681, 684, 690, 709 |
| `mw-web-admin/src/pages/Control/pages.tsx` | 68 |
| `mw-web-admin/src/preferences/preferences-context.tsx` | 553, 561 |
| `scripts/e2e-step-05.ps1` | 72 |
| `scripts/e2e-step-06.ps1` | 73 |
| `scripts/e2e-step-09.ps1` | 104, 121 |
| `scripts/e2e-step-10.ps1` | 139 |
| `scripts/e2e-step-11.ps1` | 139 |
| `scripts/e2e-step-12.ps1` | 173, 198, 239, 243 |
| `scripts/register-step-10-oauth-resource.ps1` | 33 |
| `scripts/register-step-11-mail-resource.ps1` | 36 |
| `scripts/sanity-step-03.ps1` | 85 |
| `scripts/sanity-step-04.ps1` | 211 |
| `scripts/sanity-step-13.ps1` | 26, 27 |

## Access eligibility

| Baseline file | Matching lines |
| --- | --- |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 336 |
| `docs/MW-Identity-application-integration.md` | 76 |
| `docs/dev_docs/federated-signin-verification.md` | 48 |
| `mw-identity/cmd/create-application/main.go` | 44, 45, 46, 167 |
| `mw-identity/internal/applications/model.go` | 39, 64 |
| `mw-identity/internal/applications/service.go` | 28, 67, 88, 90, 157, 184 |
| `mw-identity/internal/applications/service_test.go` | 23, 79, 80 |
| `mw-identity/internal/httpapi/consent_test.go` | 24 |
| `mw-identity/internal/httpapi/identity_admin.go` | 232, 256 |
| `mw-identity/internal/identityadmin/model.go` | 37 |
| `mw-identity/internal/identityadmin/service.go` | 30, 167, 657, 668, 669, 671 |
| `mw-identity/internal/oidc/admin_access.go` | 148 |
| `mw-identity/internal/oidc/admin_access_test.go` | 17, 18, 19, 20, 26 |
| `mw-identity/internal/store/sqlite/account_lifecycle_test.go` | 55 |
| `mw-identity/internal/store/sqlite/applications.go` | 18, 19, 46, 61, 177, 195 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 110, 164 |
| `mw-identity/internal/store/sqlite/consent.go` | 58 |
| `mw-identity/internal/store/sqlite/consent_test.go` | 46, 132 |
| `mw-identity/internal/store/sqlite/identity_admin.go` | 124, 133, 153 |
| `mw-identity/internal/store/sqlite/oidc.go` | 68, 134, 205 |
| `mw-identity/migrations/011_application_access_policy.sql` | 2, 3 |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx` | 38, 57, 131, 144, 420, 422, 424, 427 |
| `mw-web-admin/src/pages/Control/ResourcePage.tsx` | 92, 305, 631, 634 |
| `mw-web-admin/src/pages/Control/pages.tsx` | 63 |

## Authentication evidence

| Baseline file | Matching lines |
| --- | --- |
| `docs/MW-Bff-implementation-guide.md` | 149, 190, 456, 476, 508 |
| `docs/MW-Control-Plane-v1.0-documentation.md` | 427, 562 |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 148, 149, 161, 163, 177, 184, 188, 190, 309, 311, 335 |
| `docs/MW-Identity-Implementation-guide.md` | 45, 423, 617, 627, 632, 640, 683, 1157, 1281 |
| `docs/MW-Identity-application-integration.md` | 56, 84 |
| `docs/construction_steps/step-04-authorization-code-pkce.md` | 38 |
| `docs/construction_steps/step-05-bff-oidc-login.md` | 98, 147 |
| `docs/construction_steps/step-09-delegated-downstream-tokens.md` | 16, 61 |
| `docs/construction_steps/step-12-strong-auth-totp-recovery.md` | 24 |
| `docs/dev_docs/federation.md` | 36 |
| `docs/dev_docs/implement-federation.md` | 63, 310, 311, 403, 404, 436, 437, 824, 863, 864, 876 |
| `mw-bff/internal/httpapi/auth.go` | 145, 146, 214, 215 |
| `mw-bff/internal/httpapi/strong_auth.go` | 12, 15, 20 |
| `mw-bff/internal/httpapi/strong_auth_test.go` | 13, 18, 23 |
| `mw-bff/internal/identityclient/client.go` | 216, 314, 323, 324 |
| `mw-bff/internal/identityclient/jwt.go` | 31, 32, 55, 56, 210, 211, 214, 228, 229, 234, 235 |
| `mw-bff/internal/identityclient/model.go` | 35, 36 |
| `mw-bff/internal/session/service.go` | 32, 33, 46, 47, 188, 189 |
| `mw-bff/internal/store/sqlite/admin_auth.go` | 215, 231, 232, 264, 296, 316 |
| `mw-bff/migrations/002_admin_oidc_session.sql` | 25 |
| `mw-identity/internal/auth/service.go` | 259, 260, 336, 337 |
| `mw-identity/internal/auth/store.go` | 60, 61, 72, 73 |
| `mw-identity/internal/httpapi/consent.go` | 91 |
| `mw-identity/internal/httpapi/identity_auth.go` | 508, 539, 583 |
| `mw-identity/internal/httpapi/identity_strong.go` | 50 |
| `mw-identity/internal/httpapi/oauth.go` | 70, 100, 104, 106, 170 |
| `mw-identity/internal/httpapi/strong_assurance.go` | 15, 18, 23, 24, 25, 26 |
| `mw-identity/internal/oidc/delegation.go` | 37, 38, 151, 165, 166, 258, 259, 261, 262 |
| `mw-identity/internal/oidc/discovery.go` | 19, 57, 67 |
| `mw-identity/internal/oidc/model.go` | 22, 49, 50, 82, 83 |
| `mw-identity/internal/oidc/service.go` | 127, 130, 211, 212, 325, 326, 377, 378, 450 |
| `mw-identity/internal/portal/strong.go` | 27, 28 |
| `mw-identity/internal/store/sqlite/account_lifecycle_test.go` | 78, 79 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 131, 132, 158, 159 |
| `mw-identity/internal/store/sqlite/consent_test.go` | 73, 84 |
| `mw-identity/internal/store/sqlite/identity_auth.go` | 128, 137, 138, 162, 206, 236 |
| `mw-identity/internal/store/sqlite/oidc.go` | 28, 39, 40, 63, 93, 112, 167, 177, 178, 200, 227, 245 |
| `mw-identity/migrations/005_oidc_authorization_code.sql` | 10 |
| `mw-identity/migrations/008_identity_delegation.sql` | 8 |
| `mw-identity/migrations/009_identity_strong_auth.sql` | 3, 6, 7 |
| `mw-identity/web/static/account.html` | 206 |
| `mw-identity/web/static/render.go` | 48 |
| `mw-identity/web/static/render_test.go` | 93 |
| `mw-web-admin/src/lib/api.test.ts` | 30 |
| `mw-web-admin/src/lib/control-api.ts` | 155, 165 |
| `scripts/e2e-step-09.ps1` | 241 |
| `scripts/e2e-step-12.ps1` | 320 |

## Deletion callbacks

| Baseline file | Matching lines |
| --- | --- |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 69 |
| `docs/MW-Identity-application-integration.md` | 14, 21, 25 |
| `docs/dev_docs/mw-identity.md` | 106, 111, 112 |
| `mw-identity/cmd/create-application/main.go` | 49, 170, 186 |
| `mw-identity/internal/applications/model.go` | 44, 68 |
| `mw-identity/internal/applications/service.go` | 31, 109, 161, 188 |
| `mw-identity/internal/applications/service_test.go` | 177, 189, 209, 210 |
| `mw-identity/internal/auth/account_lifecycle.go` | 115, 158 |
| `mw-identity/internal/auth/account_lifecycle_test.go` | 36, 62 |
| `mw-identity/internal/auth/store.go` | 81 |
| `mw-identity/internal/httpapi/identity_admin.go` | 235, 259 |
| `mw-identity/internal/identityadmin/model.go` | 40 |
| `mw-identity/internal/identityadmin/service.go` | 33, 170, 692, 693, 694, 698, 702 |
| `mw-identity/internal/store/sqlite/account_lifecycle.go` | 18, 21, 33 |
| `mw-identity/internal/store/sqlite/account_lifecycle_test.go` | 55 |
| `mw-identity/internal/store/sqlite/applications.go` | 51, 65, 182, 217, 220 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 47, 90, 91, 277, 283 |
| `mw-identity/internal/store/sqlite/identity_admin.go` | 125, 134 |
| `mw-identity/migrations/010_account_lifecycle.sql` | 2 |
| `mw-identity/migrations/012_application_internal.sql` | 2 |
| `mw-web-admin/src/pages/Control/IdentityDetailPages.tsx` | 43, 60, 134, 147, 478, 479, 482, 538, 539 |
| `mw-web-admin/src/pages/Control/ResourcePage.tsx` | 310 |
| `mw-web-admin/src/pages/Control/pages.tsx` | 70 |
| `scripts/e2e-step-08.ps1` | 287 |
| `scripts/sanity-step-06.ps1` | 265, 284, 324 |

## Consent and deletion API

| Baseline file | Matching lines |
| --- | --- |
| `README.md` | 11, 33, 129 |
| `docs/MW-Identity-Federation-Contract-v1.0.md` | 19, 20, 27, 38, 41, 42, 46, 47, 69, 71, 83, 94, 119, 194, 196, 198, 199, 200, 206, 207, 209, 215, 217, 225, 227, 228, 230, 238, 246, 248, 250, 252, 254, 258, 260, 262, 264, 270, 273, 274, 280, 285, 297, 298, 303, 304, 305, 306, 308, 316, 318, 319, 320, 339, 340, 341, 348, 358 |
| `docs/MW-Identity-application-integration.md` | 14, 19, 21, 74, 80, 82, 88, 90, 92, 94, 96, 98, 114 |
| `docs/construction_steps/mw_mail_integration.md` | 1, 5, 22, 25 |
| `docs/construction_steps/mw_oauth_integration.md` | 1, 5, 27 |
| `docs/construction_steps/step-04-authorization-code-pkce.md` | 53, 55, 81 |
| `docs/construction_steps/step-09-delegated-downstream-tokens.md` | 38 |
| `docs/dev_docs/backup-restore-implementation.md` | 21 |
| `docs/dev_docs/federated-signin-rollout.md` | 3, 4, 10, 15, 16, 18, 20, 22, 24, 26, 27, 33, 34, 35, 40, 42, 43, 47, 49, 50, 54, 58, 63 |
| `docs/dev_docs/federated-signin-verification.md` | 5, 11, 12, 14, 16, 18, 20, 27, 46, 47, 50, 52, 73, 76, 81, 84, 92, 103, 122, 123, 125 |
| `docs/dev_docs/mw-identity.md` | 84, 113, 258 |
| `docs/dev_docs/mw-web-admin.md` | 42 |
| `docs/dev_docs/project.md` | 365 |
| `docs/dev_docs/stackport-onboarding.md` | 270 |
| `mw-bff/internal/httpapi/strong_auth_test.go` | 40, 49 |
| `mw-identity/cmd/create-application/main.go` | 51 |
| `mw-identity/internal/applications/model.go` | 16 |
| `mw-identity/internal/applications/service_test.go` | 194 |
| `mw-identity/internal/auth/account_lifecycle.go` | 16, 18, 19, 25, 28, 31, 34, 36, 39, 40, 43, 50, 69, 74, 82, 85, 87, 90, 91, 92, 94, 100, 101, 104, 106, 109, 110, 179, 188 |
| `mw-identity/internal/auth/account_lifecycle_test.go` | 19, 20, 21, 22, 35, 37, 44, 45, 46, 47, 61, 63, 68 |
| `mw-identity/internal/auth/registration.go` | 37 |
| `mw-identity/internal/auth/service.go` | 78 |
| `mw-identity/internal/auth/store.go` | 17, 29, 85, 92, 94, 150, 151, 153 |
| `mw-identity/internal/httpapi/consent.go` | 16, 62, 64, 67, 74, 83, 86, 87, 90 |
| `mw-identity/internal/httpapi/consent_test.go` | 21, 28, 59, 72, 76, 77, 78, 80, 81, 84, 85, 86, 106, 112, 113, 114, 117, 119, 121, 162 |
| `mw-identity/internal/httpapi/identity_admin.go` | 46, 47, 48 |
| `mw-identity/internal/httpapi/identity_auth.go` | 557, 589, 597, 601, 602, 607, 611, 615 |
| `mw-identity/internal/httpapi/oauth.go` | 138, 140, 143, 163, 166 |
| `mw-identity/internal/httpapi/router.go` | 189 |
| `mw-identity/internal/identityadmin/model.go` | 18, 57 |
| `mw-identity/internal/oidc/consent.go` | 10, 16, 17, 20, 21, 30, 45, 49 |
| `mw-identity/internal/oidc/delegation.go` | 168 |
| `mw-identity/internal/oidc/model.go` | 27, 40, 75, 90, 91 |
| `mw-identity/internal/oidc/service.go` | 181, 182, 183, 203, 371 |
| `mw-identity/internal/store/sqlite/account_lifecycle.go` | 12, 14, 15, 17, 18, 21, 42, 45, 51, 76, 81, 82, 83, 90 |
| `mw-identity/internal/store/sqlite/account_lifecycle_test.go` | 32, 68, 77, 112, 122, 124, 151, 153, 163, 173, 175, 183, 185 |
| `mw-identity/internal/store/sqlite/applications_test.go` | 121, 124, 129, 148, 151, 156 |
| `mw-identity/internal/store/sqlite/consent.go` | 13, 14, 16, 27, 34, 38, 55, 59 |
| `mw-identity/internal/store/sqlite/consent_test.go` | 16, 27, 55, 57, 63, 70, 73, 81, 84, 91, 92, 94, 96, 98, 102, 104, 111, 117, 119, 125 |
| `mw-identity/internal/store/sqlite/identity_admin.go` | 325, 415, 436, 448, 464, 465, 466 |
| `mw-identity/internal/store/sqlite/identity_auth.go` | 103, 176, 194, 221, 245, 246, 247, 375, 392, 409, 410, 411 |
| `mw-identity/internal/store/sqlite/identity_lookup.go` | 29 |
| `mw-identity/internal/store/sqlite/oidc.go` | 29, 30, 42, 64, 68, 96, 134, 167, 168, 179, 201, 205, 230 |
| `mw-identity/migrations/010_account_lifecycle.sql` | 22 |
| `mw-identity/migrations/013_application_consent.sql` | 1, 2, 12, 15, 20, 25, 26, 27, 34, 37, 38, 39, 42, 43 |
| `mw-identity/web/static/index.html` | 278, 286, 312 |
| `mw-identity/web/static/indexEN.html` | 275, 283, 309 |
| `mw-identity/web/static/messages.go` | 63, 65, 91, 182, 184, 210, 216 |
| `mw-identity/web/static/portal.js` | 68 |
| `mw-web-admin/src/preferences/preferences-context.tsx` | 504, 532, 546, 548, 550, 554, 559, 561, 605 |
| `mw-web-admin/static_portal/index.html` | 188, 193, 213 |
| `mw-web-admin/static_portal/portal.js` | 55 |
| `scripts/sanity-step-06.ps1` | 419 |
