param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$env:GATEWAY_HOST_PORT = "$Port"

$Root = [System.IO.Path]::GetFullPath(
    (Join-Path $PSScriptRoot "..")
)
$Step6Env = Join-Path $Root ".env.step6.local"
$Step7Env = Join-Path $Root ".env.step7.local"

if (-not (Test-Path $Step6Env)) {
    throw ".env.step6.local is required from the passing Step 06 environment."
}

function New-Base64UrlKey {
    $Bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($Bytes)
    return [Convert]::ToBase64String($Bytes).
        TrimEnd('=').
        Replace('+', '-').
        Replace('/', '_')
}

$Existing = Get-Content $Step6Env
$Lines = [System.Collections.Generic.List[string]]::new()
foreach ($Line in $Existing) { $Lines.Add($Line) }

function Ensure-EnvLine([string]$Name, [string]$Value) {
    if (-not ($Lines | Where-Object { $_.StartsWith("$Name=") })) {
        $Lines.Add("$Name=$Value")
    }
}

Ensure-EnvLine "IDENTITY_WEBAUTHN_RP_ID" "identity.mattediworks.com"
Ensure-EnvLine "IDENTITY_WEBAUTHN_RP_NAME" "MW Identity"
Ensure-EnvLine "IDENTITY_WEBAUTHN_ORIGINS" "https://identity.mattediworks.com"
Ensure-EnvLine "IDENTITY_WEBAUTHN_CREDENTIAL_KEYS" ("1:" + (New-Base64UrlKey))
Ensure-EnvLine "IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION" "1"
Ensure-EnvLine "IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES" "5"
$Lines | Set-Content -Path $Step7Env -Encoding utf8

Write-Host "== Step 07 build/start =="
docker compose --env-file $Step7Env up -d --build
if ($LASTEXITCODE -ne 0) { throw "docker compose up failed" }

$Base = "http://127.0.0.1:$Port"
$HostName = "identity.mattediworks.com"

Write-Host "== Wait for Identity =="
$Healthy = $false
for ($i = 0; $i -lt 45; $i++) {
    & curl.exe --fail --silent --show-error `
        -H "Host: $HostName" "$Base/health" | Out-Null
    if ($LASTEXITCODE -eq 0) { $Healthy = $true; break }
    Start-Sleep -Seconds 1
}
if (-not $Healthy) {
    docker compose --env-file $Step7Env ps
    docker compose --env-file $Step7Env logs --tail=160 mw-identity
    throw "Identity did not become healthy"
}

$Login = (& curl.exe --fail --silent --show-error `
    -H "Host: $HostName" "$Base/login") -join "`n"
if ($Login -notmatch 'id="passkey-login"') {
    throw "passkey login control missing from Identity portal"
}
Write-Host "PASS passkey login portal wiring"

$OptionsRaw = (& curl.exe --fail --silent --show-error `
    -X POST `
    -H "Host: $HostName" `
    -H "Accept: application/json" `
    "$Base/api/webauthn/login/options") -join "`n"
$Options = $OptionsRaw | ConvertFrom-Json
if ([string]::IsNullOrWhiteSpace($Options.ceremony_token) -or
    $Options.ceremony_token -notlike 'mwwa_*') {
    throw "WebAuthn login ceremony token missing"
}
$OptionsText = $OptionsRaw
if ($OptionsText -notmatch '"userVerification"\s*:\s*"required"') {
    throw "WebAuthn login does not require user verification"
}
Write-Host "PASS discoverable login options + single-use ceremony token"

Write-Host ""
Write-Host "STEP 07 STATIC/API PASS"
Write-Host ""
Write-Host "Required real-browser checkpoint on the HTTPS Identity origin:"
Write-Host "  1. Password login"
Write-Host "  2. Register Windows Hello/passkey on /account"
Write-Host "  3. Logout"
Write-Host "  4. Sign in with a passkey"
Write-Host "  5. Confirm /account is authenticated"
Write-Host ""
Write-Host "Only then record: STEP 07 E2E PASS"
