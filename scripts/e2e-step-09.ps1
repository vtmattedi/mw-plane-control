param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$BaseEnvCandidates = @(
    (Join-Path $Root ".env.step8.local"),
    (Join-Path $Root ".env.step7.local"),
    (Join-Path $Root ".env.step6.local"),
    (Join-Path $Root ".env.step3.local")
)
$BaseEnv = $BaseEnvCandidates |
    Where-Object { Test-Path $_ } |
    Select-Object -First 1
if ($null -eq $BaseEnv) {
    throw "Step 09 needs a previously passing Step 03/06/07/08 local env containing the persistent Identity keyrings."
}
Write-Host "Step 09 base env: $BaseEnv"
$Step9Env = Join-Path $Root ".env.step9.local"
$ComposeArgs = @(
    "compose",
    "-f", (Join-Path $Root "docker-compose.yml"),
    "-f", (Join-Path $Root "docker-compose.step09.yml"),
    "--env-file", $Step9Env
)

function Read-EnvValue([string]$Path, [string]$Name, [bool]$Required = $true) {
    $Prefix = "$Name="
    $Line = Get-Content $Path |
        Where-Object { $_.StartsWith($Prefix) } |
        Select-Object -First 1
    if ($null -eq $Line) {
        if ($Required) { throw "$Name missing from $Path" }
        return ""
    }
    return $Line.Substring($Prefix.Length)
}

function New-Base64UrlKey {
    $Bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($Bytes)
    return [Convert]::ToBase64String($Bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Request-Redirect([string]$HostName, [string]$Path, [string]$CookieJar, [string]$Headers) {
    $BodyPath = "$Headers.body"
    try {
        $Status = & curl.exe --silent --show-error --output $BodyPath --dump-header $Headers --write-out "%{http_code}" `
            -b $CookieJar -c $CookieJar -H "Host: $HostName" "http://127.0.0.1:$Port$Path"
        if ($LASTEXITCODE -ne 0) { throw "request failed: $HostName$Path" }

        $Line = Get-Content $Headers |
            Where-Object { $_ -match '^Location:\s*(.+)\s*$' } |
            Select-Object -First 1

        $BodyText = ""
        if (Test-Path $BodyPath) {
            $BodyText = (Get-Content $BodyPath -Raw -ErrorAction SilentlyContinue).Trim()
        }

        if ($null -eq $Line) {
            & docker @ComposeArgs logs --tail=120 mw-bff mw-identity
            throw "Location missing: $HostName$Path status=$Status body=$BodyText"
        }

        return @{
            Status = [int]$Status
            Location = [regex]::Match($Line, '^Location:\s*(.+?)\s*$').Groups[1].Value
            BodyText = $BodyText
        }
    }
    finally {
        if (Test-Path $BodyPath) {
            Remove-Item -LiteralPath $BodyPath -ErrorAction SilentlyContinue
        }
    }
}

function Path-And-Query([string]$URL) {
    return ([System.Uri]$URL).PathAndQuery
}

$IdentityKeys = Read-EnvValue $BaseEnv "IDENTITY_SIGNING_KEYS"
$IdentityKeyVersion = Read-EnvValue $BaseEnv "IDENTITY_SIGNING_ACTIVE_KEY_VERSION"
$WebAuthnKeys = Read-EnvValue $BaseEnv "IDENTITY_WEBAUTHN_CREDENTIAL_KEYS"
$WebAuthnKeyVersion = Read-EnvValue $BaseEnv "IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION"

$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$BFFClientID = "mw_admin_step09_$Stamp"
$RSClientID = "test-rs-step09-$Stamp"
$CallbackURL = "https://admin.mattediworks.com/api/auth/callback"

Push-Location $Root
try {
    Write-Host "== Step 09 prepare Identity applications =="
    docker compose --env-file $BaseEnv build mw-identity
    if ($LASTEXITCODE -ne 0) { throw "mw-identity build failed" }

    $BFFCreatedRaw = docker compose --env-file $BaseEnv run --rm --entrypoint /app/create-application mw-identity `
        --name "MW Admin Step 09" `
        --client-id $BFFClientID `
        --type confidential `
        --redirect-uri $CallbackURL `
        --scope openid `
        --scope profile `
        --scope email `
        --scope identity.admin `
        --scope test.read `
        --audience mw-identity-admin `
        --audience $RSClientID `
        --json
    if ($LASTEXITCODE -ne 0) { throw "create Step 09 BFF application failed" }
    $BFFCreated = ($BFFCreatedRaw -join "`n") | ConvertFrom-Json

    $RSCreatedRaw = docker compose --env-file $BaseEnv run --rm --entrypoint /app/create-application mw-identity `
        --name "Step 09 Test Resource Server" `
        --client-id $RSClientID `
        --type confidential `
        --redirect-uri "https://step09-rs.invalid/callback" `
        --scope openid `
        --scope test.read `
        --audience $RSClientID `
        --json
    if ($LASTEXITCODE -ne 0) { throw "create Step 09 resource-server application failed" }
    $RSCreated = ($RSCreatedRaw -join "`n") | ConvertFrom-Json

    if ([string]::IsNullOrWhiteSpace($BFFCreated.client_secret) -or
        [string]::IsNullOrWhiteSpace($RSCreated.client_secret)) {
        throw "Step 09 application secret missing"
    }

    $TransactionKey = New-Base64UrlKey
    @"
APP_ENV=development
GATEWAY_HOST_PORT=$Port

IDENTITY_PUBLIC_ISSUER=https://identity.mattediworks.com
IDENTITY_SIGNING_KEYS=$IdentityKeys
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=$IdentityKeyVersion
IDENTITY_WEBAUTHN_RP_ID=identity.mattediworks.com
IDENTITY_WEBAUTHN_RP_NAME=MW Identity
IDENTITY_WEBAUTHN_ORIGINS=https://identity.mattediworks.com
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS=$WebAuthnKeys
IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION=$WebAuthnKeyVersion
IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES=5
IDENTITY_DELEGATED_TOKEN_TTL_MINUTES=5

BFF_PUBLIC_BASE_URL=https://admin.mattediworks.com
BFF_IDENTITY_ISSUER=https://identity.mattediworks.com
BFF_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
BFF_OIDC_CLIENT_ID=$BFFClientID
BFF_OIDC_CLIENT_SECRET=$($BFFCreated.client_secret)
BFF_TRANSACTION_KEYS=1:$TransactionKey
BFF_TRANSACTION_ACTIVE_KEY_VERSION=1
BFF_OIDC_TRANSACTION_TTL_MINUTES=10
BFF_ADMIN_SESSION_TTL_HOURS=8
BFF_OIDC_CLOCK_SKEW_SECONDS=60
BFF_IDENTITY_HTTP_TIMEOUT_SECONDS=5
BFF_DELEGATION_SCOPES=test.read
BFF_DELEGATION_CACHE_SKEW_SECONDS=30

STEP09_RS_CLIENT_ID=$RSClientID
STEP09_RS_CLIENT_SECRET=$($RSCreated.client_secret)
STEP09_RS_AUDIENCE=$RSClientID
"@ | Set-Content -Encoding ascii $Step9Env

    Write-Host "wrote ignored local runtime env: .env.step9.local"

    & docker @ComposeArgs up -d --build
    if ($LASTEXITCODE -ne 0) { throw "Step 09 docker compose up failed" }

    $Healthy = $false
    for ($i = 0; $i -lt 50; $i++) {
        & curl.exe --fail --silent --show-error -H "Host: admin.mattediworks.com" `
            "http://127.0.0.1:$Port/api/health" | Out-Null
        if ($LASTEXITCODE -eq 0) { $Healthy = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $Healthy) {
        & docker @ComposeArgs ps
        & docker @ComposeArgs logs --tail=240
        throw "Step 09 services did not become healthy"
    }

    $Email = "step09-$Stamp@mattediworks.test"
    $Password = "Step09-Integration-Password-$Stamp"
    $Password | & docker @ComposeArgs exec -T mw-identity /app/create-superadmin `
        --email $Email --display-name "Step 09 Root" --password-stdin
    if ($LASTEXITCODE -ne 0) { throw "create-superadmin failed" }

    $CookieJar = Join-Path $env:TEMP "mw-step09-$Stamp.cookies"
    $Headers = Join-Path $env:TEMP "mw-step09-$Stamp.headers"
    try {
        $Login = Request-Redirect "admin.mattediworks.com" "/api/auth/login?return_to=%2F" $CookieJar $Headers
        if ($Login.Status -ne 302) { throw "BFF login did not redirect" }
        $AuthorizePath = Path-And-Query $Login.Location

        $IdentityAuth = Request-Redirect "identity.mattediworks.com" $AuthorizePath $CookieJar $Headers
        if ($IdentityAuth.Status -ne 303) { throw "Identity authorize did not enter login" }

        $LoginPage = (& curl.exe --fail --silent --show-error -b $CookieJar -c $CookieJar `
            -H "Host: identity.mattediworks.com" "http://127.0.0.1:$Port$($IdentityAuth.Location)") -join "`n"
        $CSRF = [regex]::Match($LoginPage, 'name="csrf_token" value="([^"]+)"').Groups[1].Value
        $ReturnToEncoded = [regex]::Match($LoginPage, 'name="return_to" value="([^"]+)"').Groups[1].Value
        $ReturnTo = [System.Net.WebUtility]::HtmlDecode($ReturnToEncoded)

        $LoginStatus = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" `
            -b $CookieJar -c $CookieJar -H "Host: identity.mattediworks.com" `
            -H "Content-Type: application/x-www-form-urlencoded" `
            --data-urlencode "email=$Email" --data-urlencode "password=$Password" `
            --data-urlencode "csrf_token=$CSRF" --data-urlencode "return_to=$ReturnTo" `
            "http://127.0.0.1:$Port/login"
        if ($LoginStatus -ne 303) { throw "Identity login failed" }

        $IdentityComplete = Request-Redirect "identity.mattediworks.com" $AuthorizePath $CookieJar $Headers
        $CallbackPath = Path-And-Query $IdentityComplete.Location
        $Callback = Request-Redirect "admin.mattediworks.com" $CallbackPath $CookieJar $Headers
        if ($Callback.Status -ne 303) { throw "BFF callback failed" }
        Write-Host "PASS human Identity -> BFF admin session"

        $MeRaw = (& curl.exe --fail --silent --show-error -b $CookieJar -c $CookieJar `
            -H "Host: admin.mattediworks.com" "http://127.0.0.1:$Port/api/auth/me") -join "`n"
        $Me = $MeRaw | ConvertFrom-Json
        $HumanSub = $Me.identity.sub
        if ([string]::IsNullOrWhiteSpace($HumanSub)) { throw "BFF human subject missing" }

        $ProbeHeaders1 = Join-Path $env:TEMP "mw-step09-probe1-$Stamp.headers"
        $Probe1Raw = (& curl.exe --fail --silent --show-error --dump-header $ProbeHeaders1 `
            -b $CookieJar -c $CookieJar -H "Host: admin.mattediworks.com" `
            "http://127.0.0.1:$Port/api/_test/delegation") -join "`n"
        if ($LASTEXITCODE -ne 0) { throw "first delegated resource call failed" }
        $Probe1 = $Probe1Raw | ConvertFrom-Json
        $Cache1 = (Get-Content $ProbeHeaders1 | Where-Object { $_ -match '^X-MW-Delegation-Cache:' } | Select-Object -First 1)
        if ($Probe1.ok -ne $true -or $Probe1.sub -ne $HumanSub -or $Probe1.aud -ne $RSClientID) {
            throw "delegated resource did not preserve human sub/exact audience"
        }
        if ($Probe1.scope -notmatch '(^|\s)test\.read($|\s)') { throw "delegated scope missing" }
        if ($Probe1.auth_time -le 0 -or $Probe1.amr -notcontains "password") {
            throw "authentication context was not retained"
        }
        if ($Cache1 -notmatch 'miss') { throw "first delegation call was not a cache miss" }
        Write-Host "PASS BFF -> token exchange -> exact-audience RS; human actor/auth context preserved"

        $ProbeHeaders2 = Join-Path $env:TEMP "mw-step09-probe2-$Stamp.headers"
        $Probe2Raw = (& curl.exe --fail --silent --show-error --dump-header $ProbeHeaders2 `
            -b $CookieJar -c $CookieJar -H "Host: admin.mattediworks.com" `
            "http://127.0.0.1:$Port/api/_test/delegation") -join "`n"
        if ($LASTEXITCODE -ne 0) { throw "second delegated resource call failed" }
        $Probe2 = $Probe2Raw | ConvertFrom-Json
        $Cache2 = (Get-Content $ProbeHeaders2 | Where-Object { $_ -match '^X-MW-Delegation-Cache:' } | Select-Object -First 1)
        if ($Probe2.sub -ne $HumanSub -or $Cache2 -notmatch 'hit') {
            throw "BFF delegated-token cache did not hit safely"
        }
        Write-Host "PASS encrypted BFF delegation cache"

        $BadStatus = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" `
            -u "${RSClientID}:$($RSCreated.client_secret)" `
            -H "Content-Type: application/x-www-form-urlencoded" `
            --data-urlencode "token=definitely-not-a-token" `
            -H "Host: identity.mattediworks.com" `
            "http://127.0.0.1:$Port/oauth/introspect"
        if ($BadStatus -ne 200) { throw "inactive-token introspection should return 200" }
        Write-Host "PASS resource-server authenticated introspection semantics"

        Write-Host ""
        Write-Host "STEP 09 E2E PASS"
    }
    finally {
        foreach ($TempPath in @($CookieJar, $Headers, $ProbeHeaders1, $ProbeHeaders2)) {
            if (-not [string]::IsNullOrWhiteSpace($TempPath)) {
                Remove-Item -LiteralPath $TempPath -ErrorAction SilentlyContinue
            }
        }
    }
}
finally {
    Pop-Location
}
