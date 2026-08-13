param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"

$env:GATEWAY_HOST_PORT = "$Port"

Write-Host "== Build and start Step 01 =="
docker compose up -d --build
if ($LASTEXITCODE -ne 0) {
    throw "docker compose up failed"
}

Write-Host ""
Write-Host "== Wait for gateway health =="

$healthy = $false
for ($i = 0; $i -lt 30; $i++) {
    try {
        & curl.exe `
            --fail `
            --silent `
            --show-error `
            "http://127.0.0.1:$Port/_gateway-health" `
            | Out-Null

        if ($LASTEXITCODE -eq 0) {
            $healthy = $true
            break
        }
    } catch {
    }

    Start-Sleep -Seconds 1
}

if (-not $healthy) {
    docker compose ps
    docker compose logs --tail=100
    throw "gateway did not become healthy in time"
}

& "$PSScriptRoot\sanity-step-01.ps1" -Port $Port
