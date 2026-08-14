$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))

Push-Location $Root
try {
    Write-Host "== Step 10 BFF tests =="
    go -C ./mw-bff test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff tests failed" }

    Write-Host "== Step 10 BFF vet =="
    go -C ./mw-bff vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff vet failed" }

    Write-Host "== Step 10 compose =="
    docker compose config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "docker compose config failed" }

    Write-Host "STEP 10 STATIC PASS"
}
finally {
    Pop-Location
}
