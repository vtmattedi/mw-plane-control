param(
    [Parameter(Mandatory=$true)][string]$BackupDirectory,
    [string]$EnvFile = ".env.step12.local"
)
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile))
$BackupDirectory = [System.IO.Path]::GetFullPath($BackupDirectory)
$ManifestPath = Join-Path $BackupDirectory "manifest.json"
if (-not (Test-Path $ManifestPath)) { throw "manifest.json missing" }
$Manifest = Get-Content $ManifestPath -Raw | ConvertFrom-Json
if ($Manifest.format -ne "mw-plane-control-backup-v1") { throw "unsupported backup format" }

Push-Location $Root
try {
    foreach ($Entry in $Manifest.files) {
        $Local = Join-Path $BackupDirectory $Entry.file
        if (-not (Test-Path $Local)) { throw "backup file missing: $($Entry.file)" }
        $Hash = (Get-FileHash -Algorithm SHA256 $Local).Hash.ToLowerInvariant()
        if ($Hash -ne [string]$Entry.sha256) { throw "checksum mismatch: $($Entry.file)" }
        $Remote = "/app/data/.step13-verify-$($Entry.file)"
        $ContainerID = (docker compose --env-file $EnvPath ps -q $Entry.service).Trim()
        if ([string]::IsNullOrWhiteSpace($ContainerID)) { throw "$($Entry.service) must be running for verification" }
        docker cp $Local "${ContainerID}:$Remote"
        if ($LASTEXITCODE -ne 0) { throw "copy into $($Entry.service) failed" }
        docker compose --env-file $EnvPath exec -T $Entry.service /app/verify-db --database $Remote
        $Code = $LASTEXITCODE
        docker compose --env-file $EnvPath exec -T $Entry.service rm -f $Remote | Out-Null
        if ($Code -ne 0) { throw "database verification failed: $($Entry.file)" }
    }
    Write-Host "STEP 13 BACKUP VERIFY PASS"
}
finally { Pop-Location }
