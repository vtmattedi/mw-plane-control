# MW Plane Control — Disaster recovery runbook

## Recovery assets

A database backup alone is intentionally insufficient. Maintain, separately and securely:

1. verified `identity.db` and `bff.db` snapshots plus `manifest.json`;
2. Identity signing-key encryption keyring and active version;
3. Identity WebAuthn credential encryption keyring and active version;
4. Identity TOTP secret encryption keyring and active version;
5. BFF transaction/session encryption keyring and active version;
6. BFF Identity confidential client registration/secret;
7. production environment/configuration for downstream MW OAuth/MW Mail federation.

Never put the keyrings into the same backup archive as the databases.

## Backup procedure

```powershell
.\scripts\backup-control-plane.ps1 -EnvFile .env.production
.\scripts\verify-step-13-backup.ps1 -BackupDirectory <new-backup> -EnvFile .env.production
```

Copy the verified backup off-host according to the operator's storage policy. Record the manifest hash independently.

## Restore decision

Prefer repairing configuration/network issues without restoring data. Restore only for actual persistence loss/corruption or an explicitly tested rollback target.

Before a destructive restore:

- preserve the current broken/unknown state;
- verify that the required external keyrings are available;
- verify the candidate backup checksum/integrity;
- understand which administrative changes occurred after the backup timestamp.

## Validation-only rehearsal

```powershell
.\scripts\restore-control-plane.ps1 `
  -BackupDirectory <backup> `
  -EnvFile .env.production `
  -ValidateOnly
```

This changes no persistent data.

## Destructive restore

Use a staging/disposable environment for regular drills. In a real incident:

```powershell
.\scripts\restore-control-plane.ps1 `
  -BackupDirectory <backup> `
  -EnvFile .env.production `
  -ConfirmRestore
```

The script first creates a pre-restore safety backup, stops gateway/BFF/Identity, replaces only the two database files through their named volumes, removes stale WAL/SHM companions, verifies database integrity/foreign keys, then restarts the services.

Never delete named volumes as a restoration technique.

## Post-restore checks

1. `docker compose ps` reports Identity/BFF/gateway healthy.
2. Identity and BFF `/ready` return `ready`.
3. JWKS contains an active usable signing key.
4. an existing expected administrator can authenticate;
5. WebAuthn/TOTP credential data decrypts using the restored environment keyrings;
6. Identity application registry/assignments are present;
7. OAuth/Mail BFF facade reads work;
8. revoke sessions/rotate credentials if compromise rather than accidental loss caused the incident.

If the database verifies but encrypted fields cannot decrypt, stop: the wrong external keyring is being used. Do not rotate/reinitialize keyrings in an attempt to “fix” restored encrypted data.
