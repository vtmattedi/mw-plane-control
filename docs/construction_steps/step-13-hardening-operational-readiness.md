# Step 13 — Hardening and operational readiness

Step 13 is the production gate for MW Plane Control. It does not introduce a new trust path. It hardens and operationalizes the Identity → BFF → delegated-service architecture proven in Steps 01–12.

## Implemented controls

### Signing keys

Identity signing rotation remains `/app/rotate-signing-key`. Rotation keeps the previous public key in JWKS. Step 13 maintenance may delete retired signing keys only after `IDENTITY_SIGNING_RETIRED_RETENTION_HOURS` (default 72h). Production validation refuses a retention window shorter than the Identity access-token TTL plus one hour.

Run rotation deliberately and observe JWKS overlap before allowing retention maintenance to remove old material.

### Confidential client credentials

Identity application client-secret rotation remains the recent-WebAuthn-protected admin operation from Steps 06/12. Rotation is a cutover operation:

1. obtain fresh WebAuthn assurance;
2. rotate the secret;
3. update the consumer secret atomically/within the maintenance window;
4. prove the new secret works;
5. prove the previous secret no longer authenticates.

Secret rotation does not silently restore or extend already-issued sessions/tokens.

### Token/session revocation strategy

The normal strategy is intentionally explicit:

- user compromise: revoke the user's Identity sessions, authorization codes and access/delegated tokens using the Step 12 revoke-sessions operation;
- catastrophic user recovery: operator recovery immediately revokes those credentials;
- application compromise: disable/archive the application as the kill switch and rotate its confidential client secret;
- delegated credentials stay short-lived and are never browser-visible;
- BFF cached delegated credentials are bounded by their upstream expiry and are removed by maintenance after expiry.

Do not add a global “accept revoked token until cache expiry” fallback.

### Backup and restore

`scripts/backup-control-plane.ps1` creates online SQLite snapshots through each service's `/app/backup-db` command. The command uses SQLite `VACUUM INTO` after a passive WAL checkpoint; it never copies a live database/WAL pair directly.

The generated `manifest.json` contains file sizes and SHA-256 hashes. It explicitly states that secret/key material is not included.

Required external material includes at minimum:

- `IDENTITY_SIGNING_KEYS`;
- `IDENTITY_WEBAUTHN_CREDENTIAL_KEYS`;
- `IDENTITY_TOTP_SECRET_KEYS`;
- `BFF_TRANSACTION_KEYS`;
- the BFF confidential OIDC client secret.

Commands:

```powershell
.\scripts\backup-control-plane.ps1 -EnvFile .env.production
.\scripts\verify-step-13-backup.ps1 -BackupDirectory <backup> -EnvFile .env.production
.\scripts\restore-control-plane.ps1 -BackupDirectory <backup> -EnvFile .env.production -ValidateOnly
```

A real restore requires `-ConfirmRestore` and first creates a new pre-restore safety backup. Never use `docker compose down -v` as part of backup, deploy, or restore.

### Retention and maintenance

Identity defaults:

```env
IDENTITY_AUDIT_RETENTION_DAYS=180
IDENTITY_SIGNING_RETIRED_RETENTION_HOURS=72
```

Maintenance deletes only data already outside its useful lifetime: expired sessions/codes/tokens, old consumed/cancelled recovery capabilities, audit rows beyond retention, and retired signing keys beyond the overlap window. BFF maintenance removes expired OIDC transactions, sessions, and delegated-token cache entries.

Schedule:

```powershell
.\scripts\maintenance-step-13.ps1 -EnvFile .env.production
```

Daily is sufficient for the expected control-plane scale.

### Rate limits and abuse controls

Identity already has dedicated login/TOTP/WebAuthn abuse limiting. Step 13 adds an outer gateway bound per-source rate limit for both Admin/BFF and Identity traffic. These are defense-in-depth controls and do not replace application authentication, CSRF, scope checks, or WebAuthn step-up.

### Request IDs and operational metrics

The gateway accepts an incoming `X-Request-ID` only when it matches a small safe character/length policy; otherwise it creates a new Nginx request ID. The normalized ID is forwarded to BFF/Identity and existing downstream OAuth/Mail clients already forward safe request IDs.

BFF and Identity also normalize request IDs internally and expose small no-secret JSON operational counters at their private service endpoint `GET /metrics`. The gateway explicitly returns 404 for the public Admin/Identity metrics paths; monitoring must run on the private control-plane network or through an operator channel.

Metrics include request count, 4xx count, 5xx count and process uptime. They contain no user/email/token/query values.

### Query-safe logging

Gateway access logging uses `$uri`, not a raw request target containing the query. This matters for OAuth callbacks and authorization requests where query values can contain one-time codes/state. Step 13 sanity fails if query-string request-target logging is reintroduced.

Application operational middleware does not log raw request URLs or query strings.

### Trusted proxy boundary

Only the gateway publishes a host port and it remains loopback-bound. BFF and Identity are private Docker-network services. The gateway normalizes forwarded protocol to `http`/`https`, passes a single trusted real-IP value as `X-Forwarded-For`, and does not preserve an arbitrary client-supplied forwarding chain.

This trust model depends on StackPort/host Nginx being the only production process allowed to reach the loopback gateway port. Do not publish BFF or Identity directly on production host interfaces.

### Health vs readiness

`/health` remains a liveness/basic database-ping endpoint.

`/ready` additionally verifies SQLite operational invariants:

- `foreign_keys=ON`;
- `journal_mode=WAL`;
- `busy_timeout >= 1000ms`.

Compose health checks use `/ready`, so a service with a reachable process but broken persistence configuration is not considered ready for dependent traffic.

### Docker hardening

Identity and BFF already run as unprivileged users. Step 13 additionally uses:

- read-only root filesystems;
- `no-new-privileges`;
- all Linux capabilities dropped for Identity/BFF;
- bounded `tmpfs` for temporary files;
- named `/app/data` volumes as the intentional writable persistence surface.

Gateway uses a read-only root filesystem, `no-new-privileges`, and bounded writable tmpfs paths required by Nginx. It does not expose BFF/Identity ports.

### Production configuration validation

Before deployment:

```powershell
.\scripts\validate-production-step-13.ps1 -EnvFile .env.production
```

Identity rejects production configuration with localhost/non-HTTPS public identity/WebAuthn origins, memory-only storage, missing encryption keyrings, or insufficient retired-signing-key overlap.

BFF rejects production configuration with localhost/non-HTTPS public URLs, Step 09 test-resource configuration, missing OAuth/Mail federation transport/host settings, or incorrect downstream audiences.

## Automated gate

```powershell
.\scripts\sanity-step-13.ps1 -EnvFile .env.step12.local
.\scripts\e2e-step-13.ps1 -EnvFile .env.step12.local
```

Expected automated marker:

```text
STEP 13 API/OPS PASS
```

It proves readiness/SQLite invariants, request-ID normalization, query-safe logging, metrics, signing-key/JWKS overlap, maintenance, online backup/integrity verification/restore validation, and Docker runtime hardening.

## Final controlled rehearsals

Do not declare the production gate complete until a disposable/staging environment records all four:

1. **Client-secret rotation:** fresh WebAuthn → rotate a disposable confidential Identity application secret → new secret accepted → old secret rejected.
2. **Revocation:** mint/use a delegated credential for a disposable admin, revoke that user's sessions/tokens, and prove introspection/use becomes inactive/rejected.
3. **Destructive restore:** make a backup, change disposable state, restore with `-ConfirmRestore`, and prove the pre-backup state plus key-dependent WebAuthn/TOTP data are usable with the unchanged external keyrings.
4. **Manual Identity recovery:** execute `docs/runbook-identity-recovery.md` for a disposable administrator and prove old sessions/passkeys are no longer the recovery trust path.

After these are recorded:

```text
STEP 13 E2E PASS
Steps 01–13 ✅
```
