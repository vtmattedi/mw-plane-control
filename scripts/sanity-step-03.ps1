param(
    [int]$Port = 3050,
    [Parameter(Mandatory = $true)]
    [string]$EnvFile
)

$ErrorActionPreference = "Stop"
$Base = "http://127.0.0.1:$Port"
$IdentityHost = "identity.mattediworks.com"

Write-Host "== Step 03 OIDC metadata / registry sanity =="

$DiscoveryRaw = & curl.exe `
    --fail `
    --silent `
    --show-error `
    -H "Host: $IdentityHost" `
    "$Base/.well-known/openid-configuration"

if ($LASTEXITCODE -ne 0) {
    throw "discovery request failed"
}

$Discovery = ($DiscoveryRaw -join "`n") | ConvertFrom-Json

if ($Discovery.issuer -ne "https://identity.mattediworks.com") {
    throw "unexpected issuer: $($Discovery.issuer)"
}
if (
    $Discovery.authorization_endpoint -ne
    "https://identity.mattediworks.com/oauth/authorize"
) {
    throw "unexpected authorization_endpoint"
}
if (
    $Discovery.token_endpoint -ne
    "https://identity.mattediworks.com/oauth/token"
) {
    throw "unexpected token_endpoint"
}
if (
    $Discovery.jwks_uri -ne
    "https://identity.mattediworks.com/.well-known/jwks.json"
) {
    throw "unexpected jwks_uri"
}

Write-Host "PASS OIDC discovery"

$JWKSRaw = & curl.exe `
    --fail `
    --silent `
    --show-error `
    -H "Host: $IdentityHost" `
    "$Base/.well-known/jwks.json"

if ($LASTEXITCODE -ne 0) {
    throw "JWKS request failed"
}

$JWKS = ($JWKSRaw -join "`n") | ConvertFrom-Json

if ($null -eq $JWKS.keys -or $JWKS.keys.Count -lt 1) {
    throw "JWKS contained no keys"
}

$InitialKid = $JWKS.keys[0].kid
if ([string]::IsNullOrWhiteSpace($InitialKid)) {
    throw "JWKS key has no kid"
}
if ($JWKS.keys[0].alg -ne "RS256") {
    throw "JWKS algorithm is not RS256"
}

Write-Host "PASS JWKS active RSA signing key"

$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$ClientID = "mw_admin_step03_$Stamp"

$CreateOutput = docker compose --env-file $EnvFile exec -T mw-identity `
    /app/create-application `
    --name "MW Admin Step 03" `
    --client-id $ClientID `
    --type confidential `
    --redirect-uri "https://admin.mattediworks.com/api/auth/callback" `
    --scope openid `
    --scope profile `
    --scope email `
    --audience mw-admin `
    --json

if ($LASTEXITCODE -ne 0) {
    throw "create-application CLI failed"
}

$Created = ($CreateOutput -join "`n") | ConvertFrom-Json

if ($Created.application.client_id -ne $ClientID) {
    throw "unexpected created client_id"
}
if (
    [string]::IsNullOrWhiteSpace(
        $Created.client_secret
    )
) {
    throw "confidential client secret missing"
}

Write-Host "PASS application registry + one-time client secret"

$RotateOutput = docker compose --env-file $EnvFile exec -T mw-identity `
    /app/rotate-signing-key

if ($LASTEXITCODE -ne 0) {
    throw "rotate-signing-key CLI failed"
}

Write-Host "PASS signing-key rotation command"

$JWKSAfterRaw = & curl.exe `
    --fail `
    --silent `
    --show-error `
    -H "Host: $IdentityHost" `
    "$Base/.well-known/jwks.json"

$JWKSAfter = ($JWKSAfterRaw -join "`n") | ConvertFrom-Json

if ($JWKSAfter.keys.Count -lt 2) {
    throw "rotated JWKS did not retain previous verification key"
}

$Kids = @($JWKSAfter.keys | ForEach-Object { $_.kid })
if ($Kids -notcontains $InitialKid) {
    throw "previous signing kid disappeared after rotation"
}

Write-Host "PASS JWKS retains retired verification key"

Write-Host ""
Write-Host "== Step 02 regression =="
& "$PSScriptRoot\sanity-step-02.ps1" -Port $Port

Write-Host ""
Write-Host "STEP 03 E2E PASS"
