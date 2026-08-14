param(
    [string]$ClientID = "mw-oauth"
)

$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$BaseEnvCandidates = @(
    (Join-Path $Root ".env.step8.local"),
    (Join-Path $Root ".env.step7.local"),
    (Join-Path $Root ".env.step6.local"),
    (Join-Path $Root ".env.step3.local")
)
$BaseEnv = $BaseEnvCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if ($null -eq $BaseEnv) {
    throw "A previously passing Step 03/06/07/08 local env is required to preserve the Identity signing keyring."
}
if ($ClientID -ne "mw-oauth") {
    throw "Step 10 requires resource client_id == audience == mw-oauth under the current Identity introspection contract."
}

$OutFile = Join-Path $Root ".env.step10.oauth-resource.local"

Push-Location $Root
try {
    Write-Host "Identity env: $BaseEnv"
    docker compose --env-file $BaseEnv build mw-identity
    if ($LASTEXITCODE -ne 0) { throw "mw-identity build failed" }

    $CreatedRaw = docker compose --env-file $BaseEnv run --rm --entrypoint /app/create-application mw-identity `
        --name "MW OAuth Resource Server" `
        --client-id "mw-oauth" `
        --type confidential `
        --redirect-uri "https://oauth.mattediworks.com/_identity-unused-callback" `
        --scope openid `
        --scope oauth.projects.read `
        --scope oauth.projects.write `
        --scope oauth.credentials.rotate `
        --scope oauth.policy.read `
        --scope oauth.policy.write `
        --scope oauth.audit.read `
        --scope oauth.metrics.read `
        --scope oauth.server.read `
        --audience mw-oauth `
        --json

    if ($LASTEXITCODE -ne 0) {
        throw "Could not create mw-oauth resource application. If it already exists, do not create a second registration; use the existing resource client and rotate/recover its client secret through the supported Identity admin procedure."
    }
    $Created = ($CreatedRaw -join "`n") | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($Created.client_secret)) {
        throw "Identity did not return the one-time MW OAuth resource client secret"
    }

    @"
MW_IDENTITY_INTROSPECTION_CLIENT_ID=mw-oauth
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=$($Created.client_secret)
MW_IDENTITY_AUDIENCE=mw-oauth
"@ | Set-Content -Encoding ascii $OutFile

    Write-Host "Created MW OAuth resource registration."
    Write-Host "Wrote ignored handoff secrets: .env.step10.oauth-resource.local"
    Write-Host "Give these values securely to the MW OAuth service team; do not commit this file."
}
finally {
    Pop-Location
}
