param(
    [Parameter(Mandatory=$true)][string]$BackupDirectory,
    [string]$EnvFile = ".env.step12.local",
    [switch]$ValidateOnly,
    [switch]$ConfirmRestore
)
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile))
$BackupDirectory = [System.IO.Path]::GetFullPath($BackupDirectory)
& (Join-Path $PSScriptRoot "verify-step-13-backup.ps1") -BackupDirectory $BackupDirectory -EnvFile $EnvFile
if ($LASTEXITCODE -ne 0) { throw "backup verification failed" }
if ($ValidateOnly) { Write-Host "Restore validation only; no data changed."; exit 0 }
if (-not $ConfirmRestore) { throw "Destructive restore refused. Re-run with -ConfirmRestore after reviewing the DR runbook." }

$Safety = Join-Path $Root ("backups/pre-restore-" + [DateTimeOffset]::UtcNow.ToString("yyyyMMdd-HHmmss"))
& (Join-Path $PSScriptRoot "backup-control-plane.ps1") -EnvFile $EnvFile -Destination $Safety
if ($LASTEXITCODE -ne 0) { throw "pre-restore safety backup failed" }

$IdentitySource = Join-Path $BackupDirectory "identity.db"
$BFFSource = Join-Path $BackupDirectory "bff.db"
Push-Location $Root
try {
    docker compose --env-file $EnvPath stop gateway mw-bff mw-identity
    if ($LASTEXITCODE -ne 0) { throw "could not stop control-plane services" }

    docker compose --env-file $EnvPath run --rm --no-deps `
        --volume "${IdentitySource}:/restore/source.db:ro" `
        --entrypoint sh mw-identity -c `
        "rm -f /app/data/identity.db-wal /app/data/identity.db-shm; cp /restore/source.db /app/data/identity.db"
    if ($LASTEXITCODE -ne 0) { throw "Identity restore copy failed" }

    docker compose --env-file $EnvPath run --rm --no-deps `
        --volume "${BFFSource}:/restore/source.db:ro" `
        --entrypoint sh mw-bff -c `
        "rm -f /app/data/bff.db-wal /app/data/bff.db-shm; cp /restore/source.db /app/data/bff.db"
    if ($LASTEXITCODE -ne 0) { throw "BFF restore copy failed" }

    docker compose --env-file $EnvPath run --rm --no-deps --entrypoint /app/verify-db mw-identity --database /app/data/identity.db
    if ($LASTEXITCODE -ne 0) { throw "restored Identity DB failed verification" }
    docker compose --env-file $EnvPath run --rm --no-deps --entrypoint /app/verify-db mw-bff --database /app/data/bff.db
    if ($LASTEXITCODE -ne 0) { throw "restored BFF DB failed verification" }

    docker compose --env-file $EnvPath up -d mw-identity mw-bff gateway
    if ($LASTEXITCODE -ne 0) { throw "restart after restore failed; safety backup is $Safety" }
    Write-Host "restore complete"
    Write-Host "pre-restore safety backup: $Safety"
    Write-Host "STEP 13 RESTORE PASS"
}
finally { Pop-Location }
