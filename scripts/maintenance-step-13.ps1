param(
    [string]$EnvFile = ".env.step12.local",
    [int]$AuditRetentionDays = 180,
    [int]$RetiredSigningRetentionHours = 72
)
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile))
if (-not (Test-Path $EnvPath)) { throw "env file not found: $EnvPath" }
Push-Location $Root
try {
    docker compose --env-file $EnvPath exec -T mw-identity /app/maintenance `
        --audit-retention-days $AuditRetentionDays `
        --retired-signing-retention-hours $RetiredSigningRetentionHours
    if ($LASTEXITCODE -ne 0) { throw "Identity maintenance failed" }
    docker compose --env-file $EnvPath exec -T mw-bff /app/maintenance
    if ($LASTEXITCODE -ne 0) { throw "BFF maintenance failed" }
    Write-Host "STEP 13 MAINTENANCE PASS"
}
finally { Pop-Location }
