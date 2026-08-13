$ErrorActionPreference = "Stop"

Write-Host "== Step 07 static sanity =="
Push-Location (Join-Path $PSScriptRoot "..\\mw-identity")
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-identity go test failed" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-identity go vet failed" }
} finally {
    Pop-Location
}

Push-Location (Join-Path $PSScriptRoot "..\\mw-bff")
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff regression go test failed" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff regression go vet failed" }
} finally {
    Pop-Location
}

Write-Host "STEP 07 STATIC PASS"
