# MW Plane Control

A shared identity platform and administrative control plane for the MattediWorks apps. It combines an OpenID Connect provider, an admin backend-for-frontend, a React console and an nginx gateway. Applications use the same identity service while retaining their own business data.

[Ecosystem case study](https://mwecosystem.mattediworks.com/) · [Application integration](docs/MW-Identity-application-integration.md) · [Development and deployment](docs/dev_docs/deployment-compose.md)

## Components and boundaries

| Component | Responsibility |
| --- | --- |
| `mw-identity/` | OIDC provider, user credentials, sessions, client/application registry, consent and delegated access |
| `mw-bff/` | Admin browser sessions and server-side calls to Identity and downstream service admin APIs |
| `mw-web-admin/` | React/TypeScript administration console |
| `infra/gateway/` | nginx routing for the console, BFF API and Identity portal |

```text
Application -------- Authorization Code + PKCE --------> MW Identity
Browser ------------ admin session -------------------> Gateway / MW BFF
MW BFF ------------- delegated tokens -----------------> Mail / OAuth / MQTT admin APIs
Downstream services -- authenticated introspection ----> MW Identity
```

Identity owns the stable account identifier (`sub`) and authentication state. Each consuming application maps that identifier to its own account and stores its own domain data. Mail delivery and Google OAuth exchange live in separate service repositories:

- [MW Mail](https://github.com/vtmattedi/mwemailserver): project-authenticated email gateway backed by Resend.
- [MW OAuth](https://github.com/vtmattedi/mwoauthservice): Google authorization broker with signed callbacks.

The console also has an optional MQTT administration facade, enabled through the BFF's MQTT resource-server configuration.

## Implemented capabilities

- Authorization Code with S256 PKCE, discovery, JWKS and registered redirect URI checks.
- Public and workforce accounts, application assignments, connected applications and coordinated account deletion callbacks.
- Passkeys/WebAuthn, TOTP and fresh-authentication checks for strong administrative operations. Password support remains where the account/application policy permits it.
- Opaque delegated tokens with audience/scope boundaries for downstream administration; service credentials stay in server-side components.
- SQLite persistence, signing-key rotation, audit events, readiness/health checks and database backup/restore tooling.
- Operator-assisted recovery that revokes existing credentials/sessions and issues a one-use recovery capability for enrolling a replacement passkey. This relies on the operator's identity verification; mailbox access alone does not complete recovery.

See [Identity implementation](docs/MW-Identity-Implementation-guide.md) and [BFF implementation](docs/MW-Bff-implementation-guide.md) for the mechanics and contracts.

## Local development

Requires Docker with Compose, Node.js 24 for the admin tooling, and the Go version declared in each backend module (currently 1.26.5) when running Go checks outside containers.

### Configure and bootstrap

Copy the root [.env.example](.env.example) to `.env` (`cp .env.example .env`, or `Copy-Item .env.example .env` in PowerShell). It documents the required encryption keyrings, public URLs, OIDC client credentials and downstream permissions.

A fresh installation needs configuration and initial Identity records:

1. Generate independent versioned encryption keys for Identity signing, WebAuthn, TOTP and BFF transaction/session storage. Replace the example placeholders; retain the keys with the corresponding persistent data.
2. Apply the example's **localhost overrides** for the local issuer, WebAuthn origins and BFF public URL. Production domain values are not interchangeable with localhost.
3. Start Identity on its own so the bootstrap commands can reach its configured database:

```sh
docker compose -f docker-compose-dev.yml up -d --build mw-identity
```

Then bootstrap the Identity administrator and register the BFF confidential client in that service's container. Its client ID, issued secret, redirect URI, scopes and audiences must match the BFF environment. Copying `.env` does not create that registration.

The [onboarding runbook](docs/dev_docs/stackport-onboarding.md#2-fresh-identity-database-root-and-admin-client) documents the bootstrap commands using production domains; use your configured localhost URLs for local development. The [deployment guide](docs/dev_docs/deployment-compose.md) describes the local topology. Keep existing databases/keyrings intact when working with an already bootstrapped installation.

After recording the issued client secret in `.env`, start the remaining backend development stack from the repository root:

```sh
docker compose -f docker-compose-dev.yml up -d --build
```

Then run the admin SPA natively:

```sh
cd mw-web-admin
cp .env.example .env
```

Set its local environment values:

```dotenv
VITE_ADMIN_API_BASE_URL=http://localhost:5173/api
VITE_IDENTITY_PUBLIC_URL=http://localhost:7100
VITE_EMAIL_API_BASE_URL=http://localhost:8082
```

```sh
npm ci
npm run dev
```

The admin SPA uses `http://localhost:5173`; its `/api` proxy reaches the gateway at `http://127.0.0.1:3050`. Identity is available at `http://localhost:7100/login`. `VITE_*` configuration is public browser/build-time configuration and must not contain secrets.

The development topology expects separately running OAuth and Mail services on host ports `8282` and `8082`, respectively, unless overridden. Their own Compose files expose container ports without publishing them; use their documented direct Go execution with the matching `PORT`, or configure reachable service URLs. The core Identity/BFF stack is not an automatic deployment of every downstream service.

## Deployment and operations

For Stackport, explicitly select [docker-compose.stackport.yml](docker-compose.stackport.yml). It builds the admin SPA into the gateway and exposes container ports without publishing host ports. The default Compose and development Compose files are local configurations.

The gateway's current virtual hosts are `admin.mattediworks.com` and `identity.mattediworks.com`. Both route to `gateway:8080`; changing domains also requires updating nginx and the corresponding public issuer, client callbacks, origins and frontend build values.

- [Deployment guide](docs/dev_docs/deployment-compose.md)
- [Bootstrap and downstream service registration](docs/dev_docs/stackport-onboarding.md)
- [Database backup](docs/dev_docs/runbook-product-backup.md)
- [Disaster recovery](docs/dev_docs/runbook-disaster-recovery.md)
- [Identity recovery](docs/dev_docs/runbook-identity-recovery.md)

Database backups and external encryption keyrings are both needed for recovery. A source checkout or container rebuild does not recover runtime accounts and encrypted records. Run destructive restore/recovery checks against disposable data first.

## Checks

From the repository root:

```sh
cd mw-identity
go test ./...
cd ../mw-bff
go test ./...
cd ../mw-web-admin
npm ci
npm run typecheck
npm run test:run
npm run build
```

The `scripts/sanity-step-*` and `scripts/e2e-step-*` scripts are staged runtime checks. Some create/delete accounts, rotate credentials, restore data or restart containers; they are not all read-only test commands.

## Scope and documentation

This is first-party infrastructure built around MW's application and operator workflows. No independent security audit or OpenID certification is claimed. Deploying it requires configuring the trust boundaries and operating account recovery, client registration, backups and key rotation.

Application integration requires the documented account-deletion callback contract, except for internal applications. Read [Integrating an application](docs/MW-Identity-application-integration.md) before wiring a client to Identity.

The [architecture](docs/MW-Control-Plane-v1.0-documentation.md), [implementation roadmap](docs/construction_steps/implementation-roadmap.md) and `docs/construction_steps/` record the engineering design and staged build. Some describe the original plan; the integration guide, current configuration and code define the shipped behavior.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE). Third-party dependencies retain their respective licenses.
