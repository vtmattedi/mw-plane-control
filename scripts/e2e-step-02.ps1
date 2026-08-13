param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$env:GATEWAY_HOST_PORT = "$Port"

Write-Host "== Rebuild and start Step 02 =="
docker compose up -d --build

if ($LASTEXITCODE -ne 0) {
    throw "docker compose up failed"
}

Write-Host ""
Write-Host "== Wait for Identity health =="

$Healthy = $false

for ($i = 0; $i -lt 30; $i++) {
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
    docker compose ps
    docker compose logs --tail=120
    throw "Identity did not become healthy in time"
}

& "$PSScriptRoot\sanity-step-02.ps1" -Port $Port
