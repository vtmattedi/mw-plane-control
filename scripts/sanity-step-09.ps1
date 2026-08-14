$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))

foreach ($Project in @("mw-identity", "mw-bff", "testapps\delegation-rs")) {
    Write-Host "== Step 09 static: $Project =="
    Push-Location (Join-Path $Root $Project)
    try {
        go test ./...
        if ($LASTEXITCODE -ne 0) { throw "$Project go test failed" }
        go vet ./...
        if ($LASTEXITCODE -ne 0) { throw "$Project go vet failed" }
    }
    finally {
        Pop-Location
    }
}

Push-Location $Root
try {
    docker compose -f docker-compose.yml -f docker-compose.step09.yml config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Step 09 compose config failed" }
}
finally {
    Pop-Location
}

Write-Host "STEP 09 STATIC PASS"
