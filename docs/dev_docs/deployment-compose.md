# Local development and Stackport deployment

## Local development

The development Compose stack runs the Gateway, Identity, and BFF only. The
admin SPA remains a native Vite process on port `5173`.

```powershell
docker compose -f docker-compose-dev.yml up -d --build
Set-Location mw-web-admin
npm ci
npm run dev
```

Local endpoints:

- Admin SPA: `http://localhost:5173`
- Identity portal: `http://localhost:7100/login`
- Gateway/API bridge: `http://127.0.0.1:3050`

The Vite proxy forwards `/api` to the Gateway. MW OAuth and MW Mail are expected
on the host at ports `8282` and `8082` unless `BFF_OAUTH_BASE_URL` and
`BFF_MAIL_BASE_URL` override them.

The local `.env` must contain valid encryption keys and the Identity OIDC client
credentials used by the BFF. Existing data remains in the named
`mw-plane-control-dev_identity-data` and `mw-plane-control-dev_bff-data`
volumes.

## Stackport

Stackport must use `docker-compose.stackport.yml`. Its Gateway image runs the
Vite production build and copies `dist` into Nginx. The Identity Docker build
compiles the HTML/CSS/JS/SVG portal into the Go server through `go:embed`.

Configure Stackport with:

- Name: `mw-plane-control`
- Compose file: `docker-compose.stackport.yml`
- Public route service: `gateway`
- Public route container port: `8080` (HTTP)
- Primary domain: `admin.mattediworks.com`
- Health endpoint: `/_gateway-health`
- Health interval: `30` seconds
- Environment file: `.env`

Use the heavily documented root `.env.example` as the canonical key/value
checklist, replacing every placeholder with a production secret. Do not commit
the resulting `.env`.

Managed Stackport projects do not allow host port publication. This Compose file
uses `expose` for Gateway (`8080`), Identity (`7100`), and BFF (`7200`), with no
`ports` entries. Both SQLite databases use persistent named volumes. The default
`docker-compose.yml` and `docker-compose-dev.yml` are local configurations; select
`docker-compose.stackport.yml` explicitly in Stackport.

The Gateway accepts both `admin.mattediworks.com` and
`identity.mattediworks.com` on container port `8080`. Configure both public routes
in the project's domain settings:

- `admin.mattediworks.com` -> `gateway` -> `8080`
- `identity.mattediworks.com` -> `gateway` -> `8080`

Stackport handles public HTTPS and certificates. Preserve the request hostname
when forwarding to the Gateway so Nginx selects the correct virtual host.

Before deployment, the production Identity application for the BFF must allow:

- Redirect URI: `https://admin.mattediworks.com/api/auth/callback`
- Scopes: `openid profile email identity.admin`
- Audience: `mw-identity-admin`

Validate the production Compose file without printing its resolved secrets, and
build the production frontend image with:

```powershell
docker compose --env-file .env -f docker-compose.stackport.yml config --quiet
docker build -f infra/gateway/Dockerfile.stackport `
  --build-arg VITE_ADMIN_API_BASE_URL=https://admin.mattediworks.com/api `
  --build-arg VITE_IDENTITY_PUBLIC_URL=https://identity.mattediworks.com `
  --build-arg VITE_EMAIL_API_BASE_URL=https://mailgo.mattediworks.com .
```
