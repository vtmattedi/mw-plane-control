param([string]$EnvFile = ".env.production")
$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$EnvPath = if ([System.IO.Path]::IsPathRooted($EnvFile)) { [System.IO.Path]::GetFullPath($EnvFile) } else { [System.IO.Path]::GetFullPath((Join-Path $Root $EnvFile)) }
if (-not (Test-Path $EnvPath)) { throw "production env file not found: $EnvPath" }
$AppEnvLine = Get-Content $EnvPath | Where-Object { $_ -match '^APP_ENV=' } | Select-Object -First 1
if ($null -eq $AppEnvLine) { throw "APP_ENV=production is required in the production env file" }
$AppEnv = $AppEnvLine.Substring('APP_ENV='.Length).Trim().ToLowerInvariant()
if ($AppEnv -notin @('production','prod')) { throw "production validation refuses APP_ENV=$AppEnv" }
Push-Location $Root
try {
    docker compose --env-file $EnvPath config | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "docker compose production config failed" }
    docker compose --env-file $EnvPath run --rm --no-deps --entrypoint /app/config-check mw-identity
    if ($LASTEXITCODE -ne 0) { throw "Identity production config invalid" }
    docker compose --env-file $EnvPath run --rm --no-deps --entrypoint /app/config-check mw-bff
    if ($LASTEXITCODE -ne 0) { throw "BFF production config invalid" }
    Write-Host "STEP 13 PRODUCTION CONFIG PASS"
}
finally { Pop-Location }
