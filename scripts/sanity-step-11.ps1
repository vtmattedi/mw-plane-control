$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))

Push-Location $Root
try {
    Write-Host "== Step 11 BFF tests =="
    go -C ./mw-bff test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff tests failed" }

    Write-Host "== Step 11 BFF vet =="
    go -C ./mw-bff vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff vet failed" }

    Write-Host "== Step 11 compose =="
    docker compose -f .\docker-compose.yml -f .\docker-compose.step11.yml config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Step 11 compose config failed" }

    Write-Host "STEP 11 STATIC PASS"
}
finally {
    Pop-Location
}
