$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
Push-Location $Root
try {
    Write-Host "== Step 12 Identity tests =="
    go -C ./mw-identity test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-identity tests failed" }
    go -C ./mw-identity vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-identity vet failed" }

    Write-Host "== Step 12 BFF tests =="
    go -C ./mw-bff test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff tests failed" }
    go -C ./mw-bff vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff vet failed" }

    Write-Host "== Step 12 compose =="
    docker compose -f .\docker-compose.yml -f .\docker-compose.step12.yml config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Step 12 compose config failed" }

    Write-Host "STEP 12 STATIC PASS"
}
finally {
    Pop-Location
}
