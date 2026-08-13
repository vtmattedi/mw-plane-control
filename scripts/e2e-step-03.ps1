param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$env:GATEWAY_HOST_PORT = "$Port"

$EnvFile = Join-Path $PSScriptRoot "..\.env.step3.local"
$EnvFile = [System.IO.Path]::GetFullPath($EnvFile)

if (-not (Test-Path $EnvFile)) {
    $Bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($Bytes)
    $Key = [Convert]::ToBase64String($Bytes).
        TrimEnd('=').
        Replace('+', '-').
        Replace('/', '_')

    @"
APP_ENV=development
GATEWAY_HOST_PORT=$Port
IDENTITY_PUBLIC_ISSUER=https://identity.mattediworks.com
IDENTITY_SIGNING_KEYS=1:$Key
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=1
"@ | Set-Content -Encoding ascii $EnvFile

    Write-Host "created local ignored env: .env.step3.local"
}
else {
    Write-Host "using existing local env: .env.step3.local"
}

Write-Host "== Rebuild and start Step 03 =="

docker compose --env-file $EnvFile up -d --build
if ($LASTEXITCODE -ne 0) {
    throw "docker compose up failed"
}

Write-Host ""
Write-Host "== Wait for Identity =="

$Healthy = $false

for ($i = 0; $i -lt 40; $i++) {
    & curl.exe `
        --fail `
        --silent `
        --show-error `
        -H "Host: identity.mattediworks.com" `
        "http://127.0.0.1:$Port/health" `
        | Out-Null

    if ($LASTEXITCODE -eq 0) {
        $Healthy = $true
        break
    }

    Start-Sleep -Seconds 1
}

if (-not $Healthy) {
    docker compose --env-file $EnvFile ps
    docker compose --env-file $EnvFile logs --tail=150
    throw "Identity did not become healthy in time"
}

& "$PSScriptRoot\sanity-step-03.ps1" `
    -Port $Port `
    -EnvFile $EnvFile
