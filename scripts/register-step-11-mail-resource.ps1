param(
    [string]$ClientID = "mw-mail"
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
if ($ClientID -ne "mw-mail") {
    throw "Step 11 requires resource client_id == audience == mw-mail under the current Identity introspection contract."
}

$OutFile = Join-Path $Root ".env.step11.mail-resource.local"
if (Test-Path $OutFile) {
    throw ".env.step11.mail-resource.local already exists. Refusing to create a duplicate mw-mail resource registration."
}

Push-Location $Root
try {
    Write-Host "Identity env: $BaseEnv"
    docker compose --env-file $BaseEnv build mw-identity
    if ($LASTEXITCODE -ne 0) { throw "mw-identity build failed" }

    $CreatedRaw = docker compose --env-file $BaseEnv run --rm --entrypoint /app/create-application mw-identity `
        --name "MW Mail Resource Server" `
        --client-id "mw-mail" `
        --type confidential `
        --internal `
        --redirect-uri "https://mailgo.mattediworks.com/_identity-unused-callback" `
        --scope openid `
        --scope mail.projects.read `
        --scope mail.projects.write `
        --scope mail.projects.delete `
        --scope mail.credentials.rotate `
        --scope mail.emails.read `
        --scope mail.usage.read `
        --scope mail.server.read `
        --audience mw-mail `
        --json

    if ($LASTEXITCODE -ne 0) {
        throw "Could not create mw-mail resource application. If it already exists, do not create a second registration; use the existing resource client and recover/rotate its secret through the supported Identity admin procedure."
    }
    $Created = ($CreatedRaw -join "`n") | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($Created.client_secret)) {
        throw "Identity did not return the one-time MW Mail resource client secret"
    }

    @"
MW_IDENTITY_INTROSPECTION_CLIENT_ID=mw-mail
MW_IDENTITY_INTROSPECTION_CLIENT_SECRET=$($Created.client_secret)
MW_IDENTITY_AUDIENCE=mw-mail
"@ | Set-Content -Encoding ascii $OutFile

    Write-Host "Created MW Mail resource registration."
    Write-Host "Wrote ignored handoff secrets: .env.step11.mail-resource.local"
    Write-Host "Use MW_IDENTITY_INTERNAL_BASE_URL=http://127.0.0.1:7100 for native Windows Mail."
}
finally {
    Pop-Location
}
