param(
    [int]$Port = 3050,
    [Parameter(Mandatory = $false)]
    [string]$MailBaseURL = $env:STEP11_MAIL_BASE_URL,
    [Parameter(Mandatory = $false)]
    [string]$MailHost = "mailgo.mattediworks.com",
    [Parameter(Mandatory = $false)]
    [string]$MailDirectURL = "http://127.0.0.1:8082"
)

$ErrorActionPreference = "Stop"
if ([string]::IsNullOrWhiteSpace($MailBaseURL)) {
    throw "Step 11 E2E requires -MailBaseURL (or STEP11_MAIL_BASE_URL) pointing to a real MW Mail hybrid/federated service reachable from the mw-bff container."
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
    throw "Step 11 requires a previously passing local env containing the persistent Identity keyrings."
}
Write-Host "Step 11 base env: $BaseEnv"

$Step11Env = Join-Path $Root ".env.step11.local"
$Step11Compose = Join-Path $Root "docker-compose.step11.yml"
if (-not (Test-Path $Step11Compose)) {
    throw "Step 11 local compose overlay missing: $Step11Compose"
}
$ComposeArgs = @(
    "compose",
    "-f", (Join-Path $Root "docker-compose.yml"),
    "-f", $Step11Compose,
    "--env-file", $Step11Env
)
$MailScopes = @(
    "mail.projects.read",
    "mail.projects.write",
    "mail.credentials.rotate",
    "mail.emails.read",
    "mail.usage.read"
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
$BFFClientID = "mw_admin_step11_$Stamp"
$CallbackURL = "https://admin.mattediworks.com/api/auth/callback"
$CookieJar = $null
$Headers = $null

Push-Location $Root
try {
    Write-Host "== Step 11 prepare BFF Identity application =="
    docker compose --env-file $BaseEnv build mw-identity
    if ($LASTEXITCODE -ne 0) { throw "mw-identity build failed" }

    $CreateArgs = @(
        "compose", "--env-file", $BaseEnv,
        "run", "--rm", "--entrypoint", "/app/create-application", "mw-identity",
        "--name", "MW Admin Step 11",
        "--client-id", $BFFClientID,
        "--type", "confidential",
        "--redirect-uri", $CallbackURL,
        "--scope", "openid",
        "--scope", "profile",
        "--scope", "email",
        "--scope", "identity.admin"
    )
    foreach ($Scope in $MailScopes) { $CreateArgs += @("--scope", $Scope) }
    $CreateArgs += @("--audience", "mw-identity-admin", "--audience", "mw-mail", "--json")
    $BFFCreatedRaw = (& docker @CreateArgs) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "create Step 11 BFF application failed" }
    $BFFCreated = $BFFCreatedRaw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace($BFFCreated.client_secret)) { throw "Step 11 BFF client secret missing" }

    $TransactionKey = New-Base64UrlKey
    $ScopeValue = $MailScopes -join " "
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

BFF_MAIL_BASE_URL=$MailBaseURL
BFF_MAIL_HOST=$MailHost
BFF_MAIL_AUDIENCE=mw-mail
BFF_MAIL_HTTP_TIMEOUT_SECONDS=5
"@ | Set-Content -Encoding ascii $Step11Env
    Write-Host "wrote ignored local runtime env: .env.step11.local"

    & docker @ComposeArgs up -d --build
    if ($LASTEXITCODE -ne 0) { throw "Step 11 docker compose up failed" }

    Write-Host "== Step 11 host Identity bridge =="
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
        throw "Step 11 control-plane services did not become healthy"
    }

    $Email = "step11-$Stamp@mattediworks.test"
    $Password = "Step11-Integration-Password-$Stamp"
    $Password | & docker @ComposeArgs exec -T mw-identity /app/create-superadmin `
        --email $Email --display-name "Step 11 Root" --password-stdin
    if ($LASTEXITCODE -ne 0) { throw "create-superadmin failed" }

    $CookieJar = Join-Path $env:TEMP "mw-step11-$Stamp.cookies"
    $Headers = Join-Path $env:TEMP "mw-step11-$Stamp.headers"

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

    $Projects = Invoke-BFF "GET" "/api/mail/projects" $CookieJar
    if ($Projects.Status -ne 200) {
        throw "MW Mail project listing through BFF failed: status=$($Projects.Status) body=$($Projects.RawBody)"
    }
    Write-Host "PASS BFF -> delegated aud=mw-mail -> MW Mail projects"

    $NoCSRF = Invoke-BFF "POST" "/api/mail/projects" $CookieJar "" @{
        name = "step11-csrf-denied-$Stamp"
        monthly_limit = 10
    }
    if ($NoCSRF.Status -ne 403) { throw "mutating Mail facade accepted missing BFF CSRF: $($NoCSRF.Status)" }
    Write-Host "PASS BFF browser-CSRF boundary"

    $Created = Invoke-BFF "POST" "/api/mail/projects" $CookieJar $BFFCSRF @{
        name = "Step 11 Federation $Stamp"
        monthly_limit = 100
    }
    if ($Created.Status -lt 200 -or $Created.Status -ge 300) {
        throw "MW Mail project create through BFF failed: status=$($Created.Status) body=$($Created.RawBody)"
    }
    $ProjectID = 0
    $CreatedAPIKey = ""
    if ($null -ne $Created.Body -and $null -ne $Created.Body.project) {
        $ProjectID = [int64]$Created.Body.project.id
        $CreatedAPIKey = [string]$Created.Body.api_key
    }
    if ($ProjectID -le 0) {
        throw "MW Mail create response did not contain project.id"
    }
    if ([string]::IsNullOrWhiteSpace($CreatedAPIKey)) {
        throw "MW Mail create response did not contain the one-time project api_key"
    }
    Write-Host "PASS delegated write scope -> real MW Mail project create"

    # Prove federation did not replace the normal project API-key boundary.
    # The intentionally invalid body must pass project authentication and fail
    # at request validation (400), without sending an email.
    $PublicBody = Join-Path $env:TEMP "mw-step11-public-$Stamp.body"
    try {
        $PublicStatus = & curl.exe --silent --show-error --output $PublicBody --write-out "%{http_code}" `
            -H "Host: $MailHost" `
            -H "Authorization: Bearer $CreatedAPIKey" `
            -H "Idempotency-Key: step11-contract-$Stamp" `
            -H "Content-Type: application/json" `
            --data-raw "{}" `
            "$($MailDirectURL.TrimEnd('/'))/v1/emails"
        if ($LASTEXITCODE -ne 0) { throw "direct MW Mail project-API probe failed" }
        if ([int]$PublicStatus -ne 400) {
            $PublicText = ""
            if (Test-Path $PublicBody) { $PublicText = (Get-Content $PublicBody -Raw -ErrorAction SilentlyContinue).Trim() }
            throw "existing /v1/emails project-key boundary was not preserved: status=$PublicStatus body=$PublicText"
        }
    }
    finally {
        Remove-Item -LiteralPath $PublicBody -ErrorAction SilentlyContinue
        $CreatedAPIKey = ""
    }
    Write-Host "PASS existing /v1/emails project API-key boundary remains independent"

    $Domains = Invoke-BFF "GET" "/api/mail/projects/$ProjectID/request-domains" $CookieJar
    if ($Domains.Status -ne 200) {
        throw "MW Mail request-domain read failed: status=$($Domains.Status) body=$($Domains.RawBody)"
    }
    $DomainValue = "step11-$Stamp.mattediworks.test"
    $DomainsPut = Invoke-BFF "PUT" "/api/mail/projects/$ProjectID/request-domains" $CookieJar $BFFCSRF @{
        allowed_request_domains = $DomainValue
    }
    if ($DomainsPut.Status -lt 200 -or $DomainsPut.Status -ge 300) {
        throw "MW Mail request-domain update failed: status=$($DomainsPut.Status) body=$($DomainsPut.RawBody)"
    }
    Write-Host "PASS delegated Mail project policy read/write"

    $Rotated = Invoke-BFF "POST" "/api/mail/projects/$ProjectID/rotate-key" $CookieJar $BFFCSRF
    if ($Rotated.Status -lt 200 -or $Rotated.Status -ge 300) {
        throw "MW Mail API-key rotation failed: status=$($Rotated.Status) body=$($Rotated.RawBody)"
    }
    if ($null -eq $Rotated.Body -or [string]::IsNullOrWhiteSpace([string]$Rotated.Body.api_key)) {
        throw "MW Mail API-key rotation response did not contain api_key"
    }
    Write-Host "PASS delegated Mail credential rotation"

    $Emails = Invoke-BFF "GET" "/api/mail/emails?limit=5" $CookieJar
    if ($Emails.Status -ne 200) {
        throw "MW Mail email-history listing failed: status=$($Emails.Status) body=$($Emails.RawBody)"
    }
    Write-Host "PASS delegated Mail email-history read"

    $Usage = Invoke-BFF "GET" "/api/mail/emails/by-project" $CookieJar
    if ($Usage.Status -ne 200) {
        throw "MW Mail usage-by-project read failed: status=$($Usage.Status) body=$($Usage.RawBody)"
    }
    Write-Host "PASS delegated Mail usage read"

    $Unmapped = Invoke-BFF "GET" "/api/mail/auth/me" $CookieJar
    if ($Unmapped.Status -ne 404) { throw "unmapped Mail facade route should be 404, got $($Unmapped.Status)" }
    Write-Host "PASS strict Mail facade deny-by-default"

    Write-Host "Human Identity sub for MW Mail structured-log correlation: $HumanSub"
    Write-Host "STEP 11 E2E PASS"
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
