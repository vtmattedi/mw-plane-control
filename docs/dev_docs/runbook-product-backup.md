# Identity recovery commands (first implementation increment)

The Identity image now contains `/app/recovery` with `create`, `validate`, and `prepare` actions. Coverage is Identity only. BFF, OAuth, Mail, scheduling, the Workspace UI, and live database replacement are not implemented in this increment.

Backups preserve original user IDs, password hashes, the global superadmin flag, applications, assignments, passkeys, TOTP credentials, signing records, and audit history. Archives use passphrase-based age encryption. Retain the recovery passphrase independently; losing it makes the archive unusable.

Retain the original `.env` securely and separately. It is not embedded in the archive. Matching issuer, WebAuthn RP/origins, and encryption key material are required. All key versions referenced by persistent credentials must remain available. The binary requires an exact match with its embedded Identity migration set in this release.

## Create

Run from the repository root with the existing Identity service running:

```powershell
docker compose --env-file .env.production exec -it -e TMPDIR=/app/data mw-identity /app/recovery create --archive /app/data/identity-recovery.age
```

Enter and confirm an independent recovery passphrase of at least 16 bytes at the terminal prompt. The command refuses an existing output file. Use a unique archive filename for subsequent backups. Snapshots do not migrate the source database. Temporary plaintext snapshots are removed when the process exits normally, including handled failures; operators must check for remnants after a forced kill or host crash. Restricted temporary directories protect these copies from other application users.

Copy the completed encrypted archive off-host, for example:

```powershell
docker compose --env-file .env.production cp mw-identity:/app/data/identity-recovery.age ./identity-recovery.age
```

Keep archives outside source control and in access-controlled storage. TMPDIR uses the persistent application volume because the service's default `/tmp` is a small tmpfs. Ensure sufficient disk space for an isolated snapshot. The database size limit is 2 GiB.

## Validate

```powershell
docker compose --env-file .env.production exec -it -e TMPDIR=/app/data mw-identity /app/recovery validate --archive /app/data/identity-recovery.age
```

Validation authenticates the entire archive, checks bounded/allowlisted entries, SQLite integrity and foreign keys, checks the schema and manifest, compares issuer/WebAuthn configuration, and decrypts persistent signing/passkey/TOTP records with the supplied environment. The JSON result includes user, global root, usable root, application, assignment, and recovery-pending counts plus required key versions. Treat this output as operationally sensitive. Validation does not alter the live database.

## Prepare an isolated restore candidate

```powershell
docker compose --env-file .env.production exec -it -e TMPDIR=/app/data mw-identity /app/recovery prepare --archive /app/data/identity-recovery.age --output-dir /app/data/recovery-prepared
```

The output directory must not already exist. Preparation validates again and requires an enabled global superadmin with a non-revoked passkey, a WebAuthn user handle, and no pending recovery. It writes only the isolated `identity.db` candidate, clears Identity sessions, authorization codes, access/delegated tokens and login ceremonies, and cancels pending recovery capabilities. Users marked recovery-pending retain that restriction.

The returned manifest describes the original backup, not a checksum for the sanitized candidate. Successful preparation does not prove a real authenticator is still available; verify fresh superadmin login during the later recovery drill.

Do not copy this candidate over a live database. The next implementation increment must provide service maintenance, BFF invalidation, a safety backup, a durable replacement journal, and rollback before exposing live restoration. Previously issued JWTs may still be accepted by offline downstream verifiers until expiry or a supported revocation procedure completes. Preparation alone is not a deployment-wide revocation guarantee.

If the original administrator has lost their authenticator, use `runbook-identity-recovery.md`. Archives without a usable root can still be created and validated, but normal preparation refuses them; exceptional operator-assisted restoration remains future work.

## Local development

With the effective Identity configuration already set in the process environment, run `go run ./cmd/recovery create|validate|prepare ...` from `mw-identity`. The command deliberately does not parse `.env`; Compose performs the existing environment-to-service mapping. Passphrases are read only from an interactive terminal and never from command-line arguments.
