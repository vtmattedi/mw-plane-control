param(
    [string]$EnvFile = ".env.step12.local",
    [string]$Destination = ""
)
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile))
if (-not (Test-Path $EnvPath)) { throw "env file not found: $EnvPath" }
$Stamp = [DateTimeOffset]::UtcNow.ToString("yyyyMMdd-HHmmss")
if ([string]::IsNullOrWhiteSpace($Destination)) {
    $Destination = Join-Path $Root "backups/control-plane-$Stamp"
}
$Destination = [System.IO.Path]::GetFullPath($Destination)
if (Test-Path $Destination) { throw "backup destination already exists: $Destination" }
New-Item -ItemType Directory -Path $Destination -Force | Out-Null

function Backup-Service([string]$Service, [string]$FileName) {
    $Remote = "/app/data/.step13-export-$FileName"
    docker compose --env-file $EnvPath exec -T $Service rm -f $Remote | Out-Null
    docker compose --env-file $EnvPath exec -T $Service /app/backup-db --output $Remote
    if ($LASTEXITCODE -ne 0) { throw "$Service online backup failed" }
    $ContainerID = (docker compose --env-file $EnvPath ps -q $Service).Trim()
    if ([string]::IsNullOrWhiteSpace($ContainerID)) { throw "$Service container is not running" }
    $Local = Join-Path $Destination $FileName
    docker cp "${ContainerID}:$Remote" $Local
    if ($LASTEXITCODE -ne 0) { throw "docker cp failed for $Service" }
    docker compose --env-file $EnvPath exec -T $Service rm -f $Remote | Out-Null
    $Hash = (Get-FileHash -Algorithm SHA256 $Local).Hash.ToLowerInvariant()
    return [ordered]@{ service=$Service; file=$FileName; bytes=(Get-Item $Local).Length; sha256=$Hash }
}

Push-Location $Root
try {
    $Files = @(
        (Backup-Service "mw-identity" "identity.db"),
        (Backup-Service "mw-bff" "bff.db")
    )
    $Manifest = [ordered]@{
        format = "mw-plane-control-backup-v1"
        created_at_utc = [DateTimeOffset]::UtcNow.ToString("o")
        secrets_included = $false
        required_external_secrets = @(
            "IDENTITY_SIGNING_KEYS",
            "IDENTITY_WEBAUTHN_CREDENTIAL_KEYS",
            "IDENTITY_TOTP_SECRET_KEYS",
            "BFF_TRANSACTION_KEYS",
            "BFF_OIDC_CLIENT_SECRET"
        )
        files = $Files
    }
    $Manifest | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 (Join-Path $Destination "manifest.json")
    Write-Host "backup: $Destination"
    Write-Host "Secrets/keyrings are intentionally NOT included."
    Write-Host "STEP 13 BACKUP PASS"
}
finally { Pop-Location }
