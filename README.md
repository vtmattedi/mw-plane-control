# MW Plane Control

MattediWorks application identity and administrative control plane.

Projects:

- `mw-identity` — Identity Provider / OIDC Authorization Server.
- `mw-bff` — Admin Backend-for-Frontend.
- `mw-web-admin` — Admin SPA source.
- `infra/gateway` — internal Nginx gateway/static host.

Architecture documentation lives in `docs/`.

Implementation is step-gated. Start with:

`docs/implementation-roadmap.md`

Local Vite development and the production Stackport Compose deployment are
documented in `docs/deployment-compose.md`.
