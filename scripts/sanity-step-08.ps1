$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))

Write-Host "== Step 08 reference RP =="
Push-Location (Join-Path $Root "testapps\account-link-rp")
try {
    $GoFiles = @(Get-ChildItem -File -Filter "*.go" | ForEach-Object { $_.FullName })

    if ($GoFiles.Count -eq 0) {
        throw "reference RP has no Go files"
    }

    & gofmt -w @GoFiles
    if ($LASTEXITCODE -ne 0) {
        throw "reference RP gofmt failed"
    }

    go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "reference RP go test failed"
    }

    go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "reference RP go vet failed"
    }
}
finally {
    Pop-Location
}

Write-Host "== MW Identity regression =="
Push-Location (Join-Path $Root "mw-identity")
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-identity go test failed" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-identity go vet failed" }
} finally {
    Pop-Location
}

Write-Host "== MW BFF regression =="
Push-Location (Join-Path $Root "mw-bff")
try {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff go test failed" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "mw-bff go vet failed" }
} finally {
    Pop-Location
}

Push-Location $Root
try {
    docker compose config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "docker compose config failed" }
} finally {
    Pop-Location
}

Write-Host "STEP 08 STATIC PASS"
