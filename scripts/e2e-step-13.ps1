param([int]$Port = 3050, [string]$EnvFile = ".env.step12.local")
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile))
if (-not (Test-Path $EnvPath)) { throw "Step 13 requires the passing Step 12 env: $EnvPath" }
$env:GATEWAY_HOST_PORT = "$Port"

function Invoke-JSON([string]$HostName, [string]$Path, [string]$RequestID = "") {
    $Args = @("--fail", "--silent", "--show-error", "-H", "Host: $HostName")
    if (-not [string]::IsNullOrWhiteSpace($RequestID)) { $Args += @("-H", "X-Request-ID: $RequestID") }
    $Args += "http://127.0.0.1:$Port$Path"
    $Raw = (& curl.exe @Args) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "request failed: $HostName$Path" }
    return $Raw | ConvertFrom-Json
}

Push-Location $Root
try {
    & .\scripts\sanity-step-13.ps1 -EnvFile $EnvFile

    docker compose --env-file $EnvPath up -d --build
    if ($LASTEXITCODE -ne 0) { throw "Step 13 runtime build/start failed" }

    $Ready = $false
    for ($i=0; $i -lt 60; $i++) {
        try {
            $B = Invoke-JSON "admin.mattediworks.com" "/api/ready"
            $I = Invoke-JSON "identity.mattediworks.com" "/ready"
            if ($B.status -eq "ready" -and $I.status -eq "ready") { $Ready=$true; break }
        } catch {}
        Start-Sleep -Seconds 1
    }
    if (-not $Ready) { docker compose --env-file $EnvPath logs --tail=240; throw "control plane did not become ready" }
    foreach ($R in @($B,$I)) {
        if (-not $R.sqlite.foreign_keys -or $R.sqlite.journal_mode -ne "wal" -or [int]$R.sqlite.busy_timeout_ms -lt 1000) {
            throw "SQLite readiness invariant failed"
        }
    }
    Write-Host "PASS health/readiness + SQLite WAL/FK/busy-timeout"

    $HeaderFile = Join-Path $env:TEMP "mw-step13-headers.txt"
    curl.exe --silent --output NUL --dump-header $HeaderFile -H "Host: admin.mattediworks.com" -H "X-Request-ID: unsafe request id" "http://127.0.0.1:$Port/api/health" | Out-Null
    $RequestID = ((Get-Content $HeaderFile | Where-Object { $_ -match '^X-Request-ID:' } | Select-Object -First 1) -replace '^X-Request-ID:\s*','').Trim()
    Remove-Item $HeaderFile -ErrorAction SilentlyContinue
    if ([string]::IsNullOrWhiteSpace($RequestID) -or $RequestID -eq "unsafe request id") { throw "request ID was not safely normalized" }
    Write-Host "PASS request-ID normalization"

    $Marker = "step13-secret-query-$([Guid]::NewGuid().ToString('N'))"
    curl.exe --silent --output NUL -H "Host: admin.mattediworks.com" "http://127.0.0.1:$Port/api/health?code=$Marker&state=$Marker" | Out-Null
    Start-Sleep -Milliseconds 150
    $Logs = (docker compose --env-file $EnvPath logs --since=30s gateway) -join "`n"
    if ($Logs.Contains($Marker)) { throw "sensitive query marker appeared in gateway logs" }
    Write-Host "PASS no-sensitive-query access logging"

    $BFFMetricsRaw = (docker compose --env-file $EnvPath exec -T mw-bff wget -qO- http://127.0.0.1:7200/metrics) -join "`n"
    $Metrics = $BFFMetricsRaw | ConvertFrom-Json
    if ([int64]$Metrics.requests_total -lt 1) { throw "BFF operational metrics did not count requests" }
    $IdentityMetricsRaw = (docker compose --env-file $EnvPath exec -T mw-identity wget -qO- http://127.0.0.1:7100/metrics) -join "`n"
    $IdentityMetrics = $IdentityMetricsRaw | ConvertFrom-Json
    if ([int64]$IdentityMetrics.requests_total -lt 1) { throw "Identity operational metrics did not count requests" }
    $PublicMetrics = & curl.exe --silent --output NUL --write-out "%{http_code}" -H "Host: admin.mattediworks.com" "http://127.0.0.1:$Port/api/metrics"
    if ([int]$PublicMetrics -ne 404) { throw "BFF metrics unexpectedly exposed through gateway" }
    Write-Host "PASS private operational metrics"

    $Before = Invoke-JSON "identity.mattediworks.com" "/.well-known/jwks.json"
    $BeforeKids = @($Before.keys | ForEach-Object { $_.kid })
    docker compose --env-file $EnvPath exec -T mw-identity /app/rotate-signing-key | Out-Host
    if ($LASTEXITCODE -ne 0) { throw "signing-key rotation failed" }
    $After = Invoke-JSON "identity.mattediworks.com" "/.well-known/jwks.json"
    $AfterKids = @($After.keys | ForEach-Object { $_.kid })
    if ($AfterKids.Count -le $BeforeKids.Count) { throw "JWKS did not retain rotation overlap" }
    foreach ($Kid in $BeforeKids) { if ($AfterKids -notcontains $Kid) { throw "previous signing key disappeared during overlap" } }
    Write-Host "PASS signing-key rotation with JWKS overlap"

    & .\scripts\maintenance-step-13.ps1 -EnvFile $EnvFile

    $TempBackup = Join-Path $env:TEMP ("mw-step13-backup-" + [Guid]::NewGuid().ToString('N'))
    & .\scripts\backup-control-plane.ps1 -EnvFile $EnvFile -Destination $TempBackup
    & .\scripts\verify-step-13-backup.ps1 -BackupDirectory $TempBackup -EnvFile $EnvFile
    & .\scripts\restore-control-plane.ps1 -BackupDirectory $TempBackup -EnvFile $EnvFile -ValidateOnly
    Remove-Item -Recurse -Force $TempBackup -ErrorAction SilentlyContinue
    Write-Host "PASS online backup + integrity verification + restore validation"

    foreach ($Service in @("mw-identity","mw-bff")) {
        $CID = (docker compose --env-file $EnvPath ps -q $Service).Trim()
        $ReadOnly = (docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' $CID).Trim()
        $Caps = (docker inspect --format '{{json .HostConfig.CapDrop}}' $CID).Trim()
        $Security = (docker inspect --format '{{json .HostConfig.SecurityOpt}}' $CID).Trim()
        if ($ReadOnly -ne "true" -or $Caps -notmatch 'ALL' -or $Security -notmatch 'no-new-privileges') { throw "$Service Docker hardening incomplete" }
    }
    $GatewayCID = (docker compose --env-file $EnvPath ps -q gateway).Trim()
    if ((docker inspect --format '{{.HostConfig.ReadonlyRootfs}}' $GatewayCID).Trim() -ne "true") { throw "gateway rootfs is not read-only" }
    Write-Host "PASS Docker runtime hardening"

    Write-Host ""
    Write-Host "STEP 13 API/OPS PASS"
    Write-Host "Final production gate still requires the controlled manual rehearsals in docs/step-13-hardening-operational-readiness.md:"
    Write-Host "  client-secret rotation/cutover; token revocation; destructive restore in staging; operator Identity recovery."
    Write-Host "After those are recorded: STEP 13 E2E PASS"
}
finally { Pop-Location }
