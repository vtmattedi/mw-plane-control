param(
    [int]$Port = 3050,
    [Parameter(Mandatory = $false)]
    [string]$OAuthBaseURL = $env:STEP10_OAUTH_BASE_URL,
    [Parameter(Mandatory = $false)]
    [string]$OAuthHost = "oauth.mattediworks.com"
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($OAuthBaseURL)) {
    throw "Step 10 E2E requires -OAuthBaseURL (or STEP10_OAUTH_BASE_URL) pointing to a real MW OAuth hybrid/federated service reachable from the mw-bff container."
}

$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$BaseEnvCandidates = @(
    (Join-Path $Root ".env.step8.local"),
    (Join-Path $Root ".env.step7.local"),
    (Join-Path $Root ".env.step6.local"),
    (Join-Path $Root ".env.step3.local")
)
$BaseEnv = $BaseEnvCandidates | Where-Object { Test-Path $_ } | Select-Object -First 1
if ($null -eq $BaseEnv) {
    throw "Step 10 requires a previously passing local env containing the persistent Identity keyrings."
}
Write-Host "Step 10 base env: $BaseEnv"

$Step10Env = Join-Path $Root ".env.step10.local"
$Step10Compose = Join-Path $Root "docker-compose.step10.yml"
if (-not (Test-Path $Step10Compose)) {
    throw "Step 10 local compose overlay missing: $Step10Compose"
}
$ComposeArgs = @(
    "compose",
    "-f", (Join-Path $Root "docker-compose.yml"),
    "-f", $Step10Compose,
    "--env-file", $Step10Env
)
$OAuthScopes = @(
    "oauth.projects.read",
    "oauth.projects.write",
    "oauth.projects.delete",
    "oauth.credentials.rotate",
    "oauth.policy.read",
    "oauth.policy.write",
    "oauth.audit.read",
    "oauth.metrics.read",
    "oauth.server.read"
)

function Read-EnvValue([string]$Path, [string]$Name, [bool]$Required = $true) {
    $Prefix = "$Name="
    $Line = Get-Content $Path | Where-Object { $_.StartsWith($Prefix) } | Select-Object -First 1
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
        $Line = Get-Content $Headers | Where-Object { $_ -match '^Location:\s*(.+)\s*$' } | Select-Object -First 1
        $BodyText = ""
        if (Test-Path $BodyPath) { $BodyText = (Get-Content $BodyPath -Raw -ErrorAction SilentlyContinue).Trim() }
        if ($null -eq $Line) { throw "Location missing: $HostName$Path status=$Status body=$BodyText" }
        return @{
            Status = [int]$Status
            Location = [regex]::Match($Line, '^Location:\s*(.+?)\s*$').Groups[1].Value
        }
    }
    finally {
        if (Test-Path $BodyPath) { Remove-Item -LiteralPath $BodyPath -ErrorAction SilentlyContinue }
    }
}

function Path-And-Query([string]$URL) {
    return ([System.Uri]$URL).PathAndQuery
}

function Invoke-BFF([string]$Method, [string]$Path, [string]$CookieJar, [string]$CSRF = "", [object]$Body = $null) {
    $Args = @(
        "--silent", "--show-error",
        "-b", $CookieJar, "-c", $CookieJar,
        "-H", "Host: admin.mattediworks.com",
        "-H", "Accept: application/json"
    )
    if ($Method -ne "GET") { $Args += @("-X", $Method) }
    if (-not [string]::IsNullOrWhiteSpace($CSRF)) { $Args += @("-H", "X-MW-CSRF: $CSRF") }
    if ($null -ne $Body) {
        $JSON = $Body | ConvertTo-Json -Depth 12 -Compress
        $Args += @("-H", "Content-Type: application/json", "--data-raw", $JSON)
    }
    $Args += @("--write-out", "`n%{http_code}", "http://127.0.0.1:$Port$Path")
    $Raw = (& curl.exe @Args) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "curl failed: $Method $Path" }
    $Split = $Raw.LastIndexOf("`n")
    if ($Split -lt 0) { throw "response status separator missing for $Method $Path" }
    $BodyText = $Raw.Substring(0, $Split)
    $Status = [int]$Raw.Substring($Split + 1)
    $Parsed = $null
    if (-not [string]::IsNullOrWhiteSpace($BodyText)) {
        try { $Parsed = $BodyText | ConvertFrom-Json } catch { $Parsed = $null }
    }
    return @{ Status = $Status; Body = $Parsed; RawBody = $BodyText }
}

$IdentityKeys = Read-EnvValue $BaseEnv "IDENTITY_SIGNING_KEYS"
$IdentityKeyVersion = Read-EnvValue $BaseEnv "IDENTITY_SIGNING_ACTIVE_KEY_VERSION"
$WebAuthnKeys = Read-EnvValue $BaseEnv "IDENTITY_WEBAUTHN_CREDENTIAL_KEYS"
$WebAuthnKeyVersion = Read-EnvValue $BaseEnv "IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION"

$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$BFFClientID = "mw_admin_step10_$Stamp"
$CallbackURL = "https://admin.mattediworks.com/api/auth/callback"
$CookieJar = $null
$Headers = $null

Push-Location $Root
try {
    Write-Host "== Step 10 prepare BFF Identity application =="
    docker compose --env-file $BaseEnv build mw-identity
    if ($LASTEXITCODE -ne 0) { throw "mw-identity build failed" }

    $CreateArgs = @(
        "compose", "--env-file", $BaseEnv,
        "run", "--rm", "--entrypoint", "/app/create-application", "mw-identity",
        "--name", "MW Admin Step 10",
        "--client-id", $BFFClientID,
        "--type", "confidential",
        "--redirect-uri", $CallbackURL,
        "--scope", "openid",
        "--scope", "profile",
        "--scope", "email",
        "--scope", "identity.admin"
    )
    foreach ($Scope in $OAuthScopes) { $CreateArgs += @("--scope", $Scope) }
    $CreateArgs += @("--audience", "mw-identity-admin", "--audience", "mw-oauth", "--json")
    $BFFCreatedRaw = (& docker @CreateArgs) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "create Step 10 BFF application failed" }
    $BFFCreated = $BFFCreatedRaw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($BFFCreated.client_secret)) { throw "Step 10 BFF client secret missing" }

    $TransactionKey = New-Base64UrlKey
    $ScopeValue = $OAuthScopes -join " "
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
BFF_DELEGATION_SCOPES=$ScopeValue
BFF_DELEGATION_CACHE_SKEW_SECONDS=30

BFF_OAUTH_BASE_URL=$OAuthBaseURL
BFF_OAUTH_HOST=$OAuthHost
BFF_OAUTH_AUDIENCE=mw-oauth
BFF_OAUTH_HTTP_TIMEOUT_SECONDS=5
"@ | Set-Content -Encoding ascii $Step10Env
    Write-Host "wrote ignored local runtime env: .env.step10.local"

    & docker @ComposeArgs up -d --build
    if ($LASTEXITCODE -ne 0) { throw "Step 10 docker compose up failed" }

    Write-Host "== Step 10 host Identity bridge =="
    $IdentityHostHealth = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" `
        "http://127.0.0.1:7100/health"
    if ($LASTEXITCODE -ne 0 -or [int]$IdentityHostHealth -ne 200) {
        throw "MW Identity is not reachable from native host services at http://127.0.0.1:7100"
    }
    Write-Host "PASS native host -> MW Identity transport"

    $Healthy = $false
    for ($i = 0; $i -lt 50; $i++) {
        & curl.exe --fail --silent --show-error -H "Host: admin.mattediworks.com" "http://127.0.0.1:$Port/api/health" | Out-Null
        if ($LASTEXITCODE -eq 0) { $Healthy = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $Healthy) {
        & docker @ComposeArgs ps
        & docker @ComposeArgs logs --tail=240
        throw "Step 10 control-plane services did not become healthy"
    }

    $Email = "step10-$Stamp@mattediworks.test"
    $Password = "Step10-Integration-Password-$Stamp"
    $Password | & docker @ComposeArgs exec -T mw-identity /app/create-superadmin `
        --email $Email --display-name "Step 10 Root" --password-stdin
    if ($LASTEXITCODE -ne 0) { throw "create-superadmin failed" }

    $CookieJar = Join-Path $env:TEMP "mw-step10-$Stamp.cookies"
    $Headers = Join-Path $env:TEMP "mw-step10-$Stamp.headers"

    $Login = Request-Redirect "admin.mattediworks.com" "/api/auth/login?return_to=%2F" $CookieJar $Headers
    if ($Login.Status -ne 302) { throw "BFF login did not redirect" }
    $AuthorizePath = Path-And-Query $Login.Location

    $IdentityAuth = Request-Redirect "identity.mattediworks.com" $AuthorizePath $CookieJar $Headers
    if ($IdentityAuth.Status -ne 303) { throw "Identity authorize did not enter login" }

    $LoginPage = (& curl.exe --fail --silent --show-error -b $CookieJar -c $CookieJar `
        -H "Host: identity.mattediworks.com" "http://127.0.0.1:$Port$($IdentityAuth.Location)") -join "`n"
    $LoginCSRF = [regex]::Match($LoginPage, 'name="csrf_token" value="([^"]+)"').Groups[1].Value
    $ReturnToEncoded = [regex]::Match($LoginPage, 'name="return_to" value="([^"]+)"').Groups[1].Value
    $ReturnTo = [System.Net.WebUtility]::HtmlDecode($ReturnToEncoded)
    if ([string]::IsNullOrWhiteSpace($LoginCSRF) -or [string]::IsNullOrWhiteSpace($ReturnTo)) {
        throw "Identity login form did not contain expected CSRF/return_to"
    }

    $LoginStatus = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" `
        -b $CookieJar -c $CookieJar -H "Host: identity.mattediworks.com" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "email=$Email" --data-urlencode "password=$Password" `
        --data-urlencode "csrf_token=$LoginCSRF" --data-urlencode "return_to=$ReturnTo" `
        "http://127.0.0.1:$Port/login"
    if ($LoginStatus -ne 303) { throw "Identity login failed" }

    $IdentityComplete = Request-Redirect "identity.mattediworks.com" $AuthorizePath $CookieJar $Headers
    $CallbackPath = Path-And-Query $IdentityComplete.Location
    $Callback = Request-Redirect "admin.mattediworks.com" $CallbackPath $CookieJar $Headers
    if ($Callback.Status -ne 303) { throw "BFF callback failed" }
    Write-Host "PASS human Identity -> BFF session"

    $Me = Invoke-BFF "GET" "/api/auth/me" $CookieJar
    if ($Me.Status -ne 200 -or [string]::IsNullOrWhiteSpace($Me.Body.identity.sub)) {
        throw "BFF human identity missing"
    }
    $HumanSub = $Me.Body.identity.sub

    $CSRFRaw = Invoke-BFF "POST" "/api/auth/csrf" $CookieJar
    if ($CSRFRaw.Status -ne 200) { throw "BFF CSRF rotation failed" }
    $BFFCSRF = $CSRFRaw.Body.csrf_token
    if ([string]::IsNullOrWhiteSpace($BFFCSRF)) { throw "BFF CSRF token missing" }

    $Projects = Invoke-BFF "GET" "/api/oauth/projects" $CookieJar
    if ($Projects.Status -ne 200) {
        throw "MW OAuth project listing through BFF failed: status=$($Projects.Status) body=$($Projects.RawBody)"
    }
    Write-Host "PASS BFF -> delegated aud=mw-oauth -> MW OAuth projects"

    $NoCSRF = Invoke-BFF "POST" "/api/oauth/projects" $CookieJar "" @{ name = "step10-csrf-denied-$Stamp"; enabled = $true }
    if ($NoCSRF.Status -ne 403) { throw "mutating OAuth facade accepted missing BFF CSRF: $($NoCSRF.Status)" }
    Write-Host "PASS BFF browser-CSRF boundary"

    $Created = Invoke-BFF "POST" "/api/oauth/projects" $CookieJar $BFFCSRF @{ name = "Step 10 Federation $Stamp"; enabled = $true }
    if ($Created.Status -lt 200 -or $Created.Status -ge 300) {
        throw "MW OAuth project create through BFF failed: status=$($Created.Status) body=$($Created.RawBody)"
    }
    Write-Host "PASS delegated write scope -> real MW OAuth operation"

    $Unmapped = Invoke-BFF "GET" "/api/oauth/auth/me" $CookieJar
    if ($Unmapped.Status -ne 404) { throw "unmapped OAuth facade route should be 404, got $($Unmapped.Status)" }
    Write-Host "PASS strict OAuth facade deny-by-default"

    $Audit = Invoke-BFF "GET" "/api/oauth/audit?limit=50" $CookieJar
    if ($Audit.Status -ne 200) {
        throw "MW OAuth audit through BFF failed: status=$($Audit.Status) body=$($Audit.RawBody)"
    }
    if ($Audit.RawBody -notmatch [regex]::Escape($HumanSub)) {
        throw "MW OAuth audit response did not expose the federated human subject $HumanSub. Confirm the OAuth team has completed actor-sub audit attribution."
    }
    Write-Host "PASS real MW OAuth audit attributes human Identity sub"

    Write-Host "STEP 10 E2E PASS"
}
finally {
    foreach ($TempPath in @($CookieJar, $Headers)) {
        if (-not [string]::IsNullOrWhiteSpace($TempPath)) {
            Remove-Item -LiteralPath $TempPath -ErrorAction SilentlyContinue
            Remove-Item -LiteralPath "$TempPath.body" -ErrorAction SilentlyContinue
        }
    }
    Pop-Location
}
