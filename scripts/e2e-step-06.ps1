param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$env:GATEWAY_HOST_PORT = "$Port"

$Step3Env = [System.IO.Path]::GetFullPath(
    (Join-Path $PSScriptRoot "..\.env.step3.local")
)
$Step6Env = [System.IO.Path]::GetFullPath(
    (Join-Path $PSScriptRoot "..\.env.step6.local")
)

if (-not (Test-Path $Step3Env)) {
    throw (
        ".env.step3.local is required because the persistent Identity " +
        "volume contains signing keys encrypted with that keyring."
    )
}

function Read-EnvValue(
    [string]$Path,
    [string]$Name
) {
    $Prefix = "$Name="
    $Line = Get-Content $Path |
        Where-Object { $_.StartsWith($Prefix) } |
        Select-Object -First 1

    if ($null -eq $Line) {
        throw "$Name missing from $Path"
    }

    return $Line.Substring($Prefix.Length)
}

function New-Base64UrlKey {
    $Bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($Bytes)

    return [Convert]::ToBase64String($Bytes).
        TrimEnd('=').
        Replace('+', '-').
        Replace('/', '_')
}

$IdentityKeys = Read-EnvValue `
    $Step3Env `
    "IDENTITY_SIGNING_KEYS"

$IdentityKeyVersion = Read-EnvValue `
    $Step3Env `
    "IDENTITY_SIGNING_ACTIVE_KEY_VERSION"

$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$ClientID = "mw_admin_step06_$Stamp"
$CallbackURL = "https://admin.mattediworks.com/api/auth/callback"

Write-Host "== Prepare Step 06 privileged BFF client =="

docker compose --env-file $Step3Env build mw-identity
if ($LASTEXITCODE -ne 0) {
    throw "mw-identity build failed"
}

$CreatedRaw = docker compose --env-file $Step3Env run --rm `
    --entrypoint /app/create-application `
    mw-identity `
    --name "MW Admin Step 06" `
    --client-id $ClientID `
    --type confidential `
    --internal `
    --redirect-uri $CallbackURL `
    --scope openid `
    --scope profile `
    --scope email `
    --scope identity.admin `
    --audience mw-identity-admin `
    --json

if ($LASTEXITCODE -ne 0) {
    throw "create privileged BFF application failed"
}

$Created = ($CreatedRaw -join "`n") | ConvertFrom-Json
$ClientSecret = $Created.client_secret

if ([string]::IsNullOrWhiteSpace($ClientSecret)) {
    throw "privileged BFF client secret missing"
}

$TransactionKey = New-Base64UrlKey

@"
APP_ENV=development
GATEWAY_HOST_PORT=$Port

IDENTITY_PUBLIC_ISSUER=https://identity.mattediworks.com
IDENTITY_SIGNING_KEYS=$IdentityKeys
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=$IdentityKeyVersion

BFF_PUBLIC_BASE_URL=https://admin.mattediworks.com
BFF_IDENTITY_ISSUER=https://identity.mattediworks.com
BFF_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
BFF_OIDC_CLIENT_ID=$ClientID
BFF_OIDC_CLIENT_SECRET=$ClientSecret
BFF_TRANSACTION_KEYS=1:$TransactionKey
BFF_TRANSACTION_ACTIVE_KEY_VERSION=1
BFF_OIDC_TRANSACTION_TTL_MINUTES=10
BFF_ADMIN_SESSION_TTL_HOURS=8
BFF_OIDC_CLOCK_SKEW_SECONDS=60
BFF_IDENTITY_HTTP_TIMEOUT_SECONDS=5
"@ | Set-Content -Encoding ascii $Step6Env

Write-Host "wrote ignored local runtime env: .env.step6.local"

Write-Host ""
Write-Host "== Rebuild and start Step 06 =="

docker compose --env-file $Step6Env up -d --build
if ($LASTEXITCODE -ne 0) {
    throw "docker compose up failed"
}

$Healthy = $false
for ($i = 0; $i -lt 40; $i++) {
    & curl.exe `
        --fail `
        --silent `
        --show-error `
        -H "Host: admin.mattediworks.com" `
        "http://127.0.0.1:$Port/api/health" |
        Out-Null

    if ($LASTEXITCODE -eq 0) {
        $Healthy = $true
        break
    }

    Start-Sleep -Seconds 1
}

if (-not $Healthy) {
    docker compose --env-file $Step6Env ps
    docker compose --env-file $Step6Env logs --tail=220
    throw "Step 06 services did not become healthy"
}

& "$PSScriptRoot\sanity-step-06.ps1" `
    -Port $Port `
    -EnvFile $Step6Env
