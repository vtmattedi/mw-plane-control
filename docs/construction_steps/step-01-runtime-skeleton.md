# Step 01 — Gateway + BFF + Identity Health E2E

## Implemented topology

```text
127.0.0.1:3050
      |
      v
gateway:8080
  |
  +-- Host admin.mattediworks.com
  |     +-- /api/* -> mw-bff:7200
  |     +-- /*     -> static placeholder
  |
  +-- Host identity.mattediworks.com
        +-- /*     -> mw-identity:7100
```

Only gateway publishes a host port.

BFF and Identity have independent SQLite databases on named volumes.

## Identity

Step 01 provides:

```text
GET /
GET /health
```

The root is deliberately plain HTML.

The service also has:

- env config;
- structured JSON logging;
- graceful shutdown;
- SQLite WAL/foreign keys/busy timeout;
- embedded SQL migrations;
- build metadata variables;
- health check including database ping.

## BFF

Step 01 provides:

```text
GET /health
```

Externally through the Admin gateway this is:

```text
GET https://admin.mattediworks.com/api/health
```

The BFF has its own independent SQLite database and migration history.

## Gateway

The gateway:

- strictly routes the two known Host values;
- returns 404 for an unknown Host;
- serves a temporary static Admin page;
- strips `/api/` before proxying to BFF;
- proxies the Identity host to MW Identity;
- preserves Host and forwarded proxy headers;
- preserves an outer `X-Forwarded-Proto` instead of replacing HTTPS with the
  inner HTTP scheme.

The outer StackPort Nginx will terminate TLS later.

## Static checks

The Step 01 generator runs:

```text
go mod tidy
gofmt
go test ./...
go vet ./...
docker compose config
```

for the relevant projects.

## Runtime E2E

PowerShell:

```powershell
.\scripts\e2e-step-01.ps1
```

Optional port:

```powershell
.\scripts\e2e-step-01.ps1 -Port 3051
```

The script leaves the stack running for inspection.

To stop it without deleting named SQLite volumes:

```powershell
docker compose down
```

Do not use:

```text
docker compose down -v
```

unless intentionally deleting Step 01 data.

## Completion criteria

Step 01 is complete only when the runtime sanity script reports:

```text
STEP 01 E2E PASS
```

Do not begin Step 02 before that checkpoint is clean.
