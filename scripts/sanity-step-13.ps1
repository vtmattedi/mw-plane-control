param([string]$EnvFile = ".env.step12.local")
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile))
if (-not (Test-Path $EnvPath)) { throw "env file not found: $EnvPath" }
Push-Location $Root
try {
    go -C .\mw-identity test ./...
    if ($LASTEXITCODE -ne 0) { throw "Identity tests failed" }
    go -C .\mw-identity vet ./...
    if ($LASTEXITCODE -ne 0) { throw "Identity vet failed" }
    go -C .\mw-bff test ./...
    if ($LASTEXITCODE -ne 0) { throw "BFF tests failed" }
    go -C .\mw-bff vet ./...
    if ($LASTEXITCODE -ne 0) { throw "BFF vet failed" }
    docker compose --env-file $EnvPath config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Compose config failed" }

    $Gateway = Get-Content .\infra\gateway\nginx.conf -Raw
    if ($Gateway -notmatch '\$request_method \$uri \$server_protocol') { throw "gateway access log is not path-only" }
    if ($Gateway -match '\$request_uri') { throw "gateway config references request_uri; query-string logging is forbidden" }
    if ($Gateway -notmatch 'X-Request-ID') { throw "gateway request-ID propagation missing" }
    if ($Gateway -notmatch 'limit_req_zone') { throw "gateway abuse rate limits missing" }

    foreach ($Path in @(
        '.\mw-identity\internal\httpapi\readiness.go',
        '.\mw-bff\internal\httpapi\readiness.go',
        '.\scripts\backup-control-plane.ps1',
        '.\scripts\restore-control-plane.ps1',
        '.\docs\dev_docs\runbook-disaster-recovery.md',
        '.\docs\dev_docs\runbook-identity-recovery.md'
    )) { if (-not (Test-Path $Path)) { throw "Step 13 artifact missing: $Path" } }

    Write-Host "STEP 13 STATIC PASS"
}
finally { Pop-Location }
