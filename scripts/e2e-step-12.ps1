param([int]$Port = 3050)

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
    throw "Step 12 requires a previously passing Identity keyring env."
}
Write-Host "Step 12 base env: $BaseEnv"

$Step12Env = Join-Path $Root ".env.step12.local"
$Step12Compose = Join-Path $Root "docker-compose.step12.yml"
if (-not (Test-Path $Step12Compose)) {
    throw "Step 12 local compose overlay missing: $Step12Compose"
}
$ComposeArgs = @(
    "compose",
    "-f", (Join-Path $Root "docker-compose.yml"),
    "-f", $Step12Compose,
    "--env-file", $Step12Env
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

function New-Base64UrlKey([int]$Size = 32) {
    $Bytes = New-Object byte[] $Size
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($Bytes)
    return [Convert]::ToBase64String($Bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function New-PKCEChallenge {
    $Verifier = New-Base64UrlKey 48
    $Bytes = [System.Text.Encoding]::ASCII.GetBytes($Verifier)
    $Digest = [System.Security.Cryptography.SHA256]::HashData($Bytes)
    return [Convert]::ToBase64String($Digest).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Request-Redirect([string]$HostName, [string]$Path, [string]$CookieJar, [string]$Headers) {
    $BodyPath = "$Headers.body"
    try {
        $Status = & curl.exe --silent --show-error --output $BodyPath --dump-header $Headers --write-out "%{http_code}" `
            -b $CookieJar -c $CookieJar -H "Host: $HostName" "http://127.0.0.1:$Port$Path"
        if ($LASTEXITCODE -ne 0) { throw "request failed: $HostName$Path" }
        $Line = Get-Content $Headers | Where-Object { $_ -match '^Location:\s*(.+)\s*$' } | Select-Object -First 1
        $BodyText = ""
        if (Test-Path $BodyPath) {
            $BodyText = (Get-Content $BodyPath -Raw -ErrorAction SilentlyContinue).Trim()
        }
        if ($null -eq $Line) {
            throw "Location missing: $HostName$Path status=$Status body=$BodyText"
        }
        return @{
            Status = [int]$Status
            Location = [regex]::Match($Line, '^Location:\s*(.+?)\s*$').Groups[1].Value
            BodyText = $BodyText
        }
    }
    finally {
        if (Test-Path $BodyPath) { Remove-Item -LiteralPath $BodyPath -ErrorAction SilentlyContinue }
    }
}

function Invoke-IdentityStatus(
    [string]$Method,
    [string]$Path,
    [string]$CookieJar = "",
    [hashtable]$Headers = @{}
) {
    $Args = @("--silent", "--show-error", "--output", "NUL", "--write-out", "%{http_code}", "-H", "Host: identity.mattediworks.com")
    if (-not [string]::IsNullOrWhiteSpace($CookieJar)) { $Args += @("-b", $CookieJar, "-c", $CookieJar) }
    if ($Method -ne "GET") { $Args += @("-X", $Method) }
    foreach ($Name in $Headers.Keys) { $Args += @("-H", "${Name}: $($Headers[$Name])") }
    $Args += "http://127.0.0.1:$Port$Path"
    $Status = & curl.exe @Args
    if ($LASTEXITCODE -ne 0) { throw "Identity request failed: $Method $Path" }
    return [int]$Status
}

function New-AuthorizePath([string]$ClientID, [string]$RedirectURI, [string]$Extra = "") {
    $Challenge = New-PKCEChallenge
    $State = New-Base64UrlKey 18
    $Nonce = New-Base64UrlKey 18
    $Path = "/oauth/authorize?response_type=code" +
        "&client_id=$([uri]::EscapeDataString($ClientID))" +
        "&redirect_uri=$([uri]::EscapeDataString($RedirectURI))" +
        "&scope=openid" +
        "&state=$([uri]::EscapeDataString($State))" +
        "&nonce=$([uri]::EscapeDataString($Nonce))" +
        "&code_challenge=$([uri]::EscapeDataString($Challenge))" +
        "&code_challenge_method=S256"
    if (-not [string]::IsNullOrWhiteSpace($Extra)) { $Path += "&$Extra" }
    return $Path
}

$IdentityKeys = Read-EnvValue $BaseEnv "IDENTITY_SIGNING_KEYS"
$IdentityKeyVersion = Read-EnvValue $BaseEnv "IDENTITY_SIGNING_ACTIVE_KEY_VERSION"
$WebAuthnKeys = Read-EnvValue $BaseEnv "IDENTITY_WEBAUTHN_CREDENTIAL_KEYS"
$WebAuthnKeyVersion = Read-EnvValue $BaseEnv "IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION"
$TOTPKeyEntry = ""
if (Test-Path $Step12Env) {
    $TOTPKeyEntry = Read-EnvValue $Step12Env "IDENTITY_TOTP_SECRET_KEYS" $false
}
if ([string]::IsNullOrWhiteSpace($TOTPKeyEntry)) {
    $TOTPKeyEntry = "1:$(New-Base64UrlKey)"
}
$TransactionKey = New-Base64UrlKey
$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$CookieJar = Join-Path $env:TEMP "mw-step12-$Stamp.cookies"
$RecoveryCookieJar = Join-Path $env:TEMP "mw-step12-recovery-$Stamp.cookies"
$Headers = Join-Path $env:TEMP "mw-step12-$Stamp.headers"

$OAuthScopes = @(
    "oauth.projects.read", "oauth.projects.write", "oauth.projects.delete", "oauth.credentials.rotate",
    "oauth.policy.read", "oauth.policy.write", "oauth.audit.read",
    "oauth.metrics.read", "oauth.server.read"
)
$MailScopes = @(
    "mail.projects.read", "mail.projects.write", "mail.projects.delete", "mail.credentials.rotate",
    "mail.emails.read", "mail.usage.read", "mail.server.read"
)
$DelegationScopes = @($OAuthScopes + $MailScopes)

Push-Location $Root
try {
    # Bootstrap a Step 12 env first so the new Identity binary/CLI can load its
    # TOTP encryption keyring before BFF credentials have been created.
    @"
APP_ENV=development
GATEWAY_HOST_PORT=$Port
IDENTITY_PUBLIC_ISSUER=http://localhost:7100
IDENTITY_SIGNING_KEYS=$IdentityKeys
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=$IdentityKeyVersion
IDENTITY_WEBAUTHN_RP_ID=localhost
IDENTITY_WEBAUTHN_RP_NAME=MW Identity
IDENTITY_WEBAUTHN_ORIGINS=http://localhost:7100
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS=$WebAuthnKeys
IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION=$WebAuthnKeyVersion
IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES=5
IDENTITY_TOTP_SECRET_KEYS=$TOTPKeyEntry
IDENTITY_TOTP_SECRET_ACTIVE_KEY_VERSION=1
IDENTITY_TOTP_ISSUER=MW Identity
IDENTITY_STRONG_AUTH_MAX_AGE_MINUTES=5
IDENTITY_RECOVERY_TTL_MINUTES=10
BFF_STRONG_AUTH_MAX_AGE_MINUTES=5
"@ | Set-Content -Encoding ascii $Step12Env

    Write-Host "== Step 12 prepare Identity applications =="
    & docker @ComposeArgs build mw-identity
    if ($LASTEXITCODE -ne 0) { throw "mw-identity build failed" }

    $BFFClientID = "mw_admin_step12_$Stamp"
    $BFFCallback = "http://localhost:7200/api/auth/callback"
    $CreateBFF = @(
        "compose", "-f", (Join-Path $Root "docker-compose.yml"), "-f", $Step12Compose,
        "--env-file", $Step12Env,
        "run", "--rm", "--entrypoint", "/app/create-application", "mw-identity",
        "--name", "MW Admin Step 12",
        "--client-id", $BFFClientID,
        "--type", "confidential",
        "--auth-policy", "strong",
        "--redirect-uri", $BFFCallback,
        "--scope", "openid", "--scope", "profile", "--scope", "email", "--scope", "identity.admin"
    )
    foreach ($Scope in $DelegationScopes) { $CreateBFF += @("--scope", $Scope) }
    $CreateBFF += @("--audience", "mw-identity-admin", "--audience", "mw-oauth", "--audience", "mw-mail", "--json")
    $BFFRaw = (& docker @CreateBFF) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "create Step 12 BFF application failed" }
    $BFFApp = $BFFRaw | ConvertFrom-Json
    if ([string]::IsNullOrWhiteSpace([string]$BFFApp.client_secret)) { throw "Step 12 BFF secret missing" }

    $StandardClientID = "step12_standard_$Stamp"
    $StrongClientID = "step12_strong_$Stamp"
    $StandardRedirect = "https://step12-standard.invalid/callback"
    $StrongRedirect = "https://step12-strong.invalid/callback"

    foreach ($Spec in @(
        @{ Name = "Step 12 Standard"; ID = $StandardClientID; Policy = "standard"; Redirect = $StandardRedirect },
        @{ Name = "Step 12 Strong"; ID = $StrongClientID; Policy = "strong"; Redirect = $StrongRedirect }
    )) {
        $Create = @(
            "compose", "-f", (Join-Path $Root "docker-compose.yml"), "-f", $Step12Compose,
            "--env-file", $Step12Env,
            "run", "--rm", "--entrypoint", "/app/create-application", "mw-identity",
            "--name", $Spec.Name, "--client-id", $Spec.ID, "--type", "confidential",
            "--auth-policy", $Spec.Policy, "--redirect-uri", $Spec.Redirect,
            "--scope", "openid", "--json"
        )
        $Raw = (& docker @Create) -join "`n"
        if ($LASTEXITCODE -ne 0) { throw "create $($Spec.Name) application failed" }
        $null = $Raw | ConvertFrom-Json
    }

    $ScopeValue = $DelegationScopes -join " "
    @"
APP_ENV=development
GATEWAY_HOST_PORT=$Port
IDENTITY_PUBLIC_ISSUER=http://localhost:7100
IDENTITY_SIGNING_KEYS=$IdentityKeys
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=$IdentityKeyVersion
IDENTITY_WEBAUTHN_RP_ID=localhost
IDENTITY_WEBAUTHN_RP_NAME=MW Identity
IDENTITY_WEBAUTHN_ORIGINS=http://localhost:7100
IDENTITY_WEBAUTHN_CREDENTIAL_KEYS=$WebAuthnKeys
IDENTITY_WEBAUTHN_CREDENTIAL_ACTIVE_KEY_VERSION=$WebAuthnKeyVersion
IDENTITY_WEBAUTHN_CEREMONY_TTL_MINUTES=5
IDENTITY_TOTP_SECRET_KEYS=$TOTPKeyEntry
IDENTITY_TOTP_SECRET_ACTIVE_KEY_VERSION=1
IDENTITY_TOTP_ISSUER=MW Identity
IDENTITY_STRONG_AUTH_MAX_AGE_MINUTES=5
IDENTITY_RECOVERY_TTL_MINUTES=10
BFF_PUBLIC_BASE_URL=http://localhost:7200
BFF_IDENTITY_ISSUER=http://localhost:7100
BFF_IDENTITY_INTERNAL_BASE_URL=http://mw-identity:7100
BFF_OIDC_CLIENT_ID=$BFFClientID
BFF_OIDC_CLIENT_SECRET=$($BFFApp.client_secret)
BFF_TRANSACTION_KEYS=1:$TransactionKey
BFF_TRANSACTION_ACTIVE_KEY_VERSION=1
BFF_OIDC_TRANSACTION_TTL_MINUTES=10
BFF_ADMIN_SESSION_TTL_HOURS=8
BFF_OIDC_CLOCK_SKEW_SECONDS=60
BFF_IDENTITY_HTTP_TIMEOUT_SECONDS=5
BFF_DELEGATION_SCOPES=$ScopeValue
BFF_DELEGATION_CACHE_SKEW_SECONDS=30
BFF_STRONG_AUTH_MAX_AGE_MINUTES=5
BFF_OAUTH_BASE_URL=http://host.docker.internal:8282
BFF_OAUTH_HOST=oauth.mattediworks.com
BFF_OAUTH_AUDIENCE=mw-oauth
BFF_OAUTH_HTTP_TIMEOUT_SECONDS=5
BFF_MAIL_BASE_URL=http://host.docker.internal:8082
BFF_MAIL_HOST=mailgo.mattediworks.com
BFF_MAIL_AUDIENCE=mw-mail
BFF_MAIL_HTTP_TIMEOUT_SECONDS=5
"@ | Set-Content -Encoding ascii $Step12Env
    Write-Host "wrote ignored local runtime env: .env.step12.local"

    & docker @ComposeArgs up -d --build
    if ($LASTEXITCODE -ne 0) { throw "Step 12 docker compose up failed" }

    $Healthy = $false
    for ($i = 0; $i -lt 50; $i++) {
        & curl.exe --fail --silent --show-error -H "Host: identity.mattediworks.com" "http://127.0.0.1:$Port/health" | Out-Null
        if ($LASTEXITCODE -eq 0) { $Healthy = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $Healthy) {
        & docker @ComposeArgs logs --tail=240 mw-identity mw-bff gateway
        throw "Step 12 control-plane services did not become healthy"
    }
    $HostHealth = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" "http://127.0.0.1:7100/health"
    if ($LASTEXITCODE -ne 0 -or [int]$HostHealth -ne 200) { throw "Step 12 localhost Identity bridge failed" }
    Write-Host "PASS Step 12 runtime + localhost Identity bridge"

    $Email = "step12-$Stamp@mattediworks.test"
    $Password = "Step12-Integration-Password-$Stamp"
    $Password | & docker @ComposeArgs exec -T mw-identity /app/create-superadmin `
        --email $Email --display-name "Step 12 Root" --password-stdin
    if ($LASTEXITCODE -ne 0) { throw "create-superadmin failed" }

    # Step 06 requires an explicit assignment for ordinary relying applications.
    # The global superadmin bootstrap exception applies only to the privileged
    # identity.admin + mw-identity-admin application, not to these disposable
    # standard/strong policy probes.
    foreach ($AssignedClientID in @($StandardClientID, $StrongClientID)) {
        & docker @ComposeArgs exec -T mw-identity /app/assign-application `
            --email $Email --client-id $AssignedClientID --role user
        if ($LASTEXITCODE -ne 0) {
            throw "Step 12 application assignment failed for $AssignedClientID"
        }
    }
    Write-Host "PASS Step 12 disposable application assignments"

    # Establish one legacy/password Identity session. Standard applications keep
    # this compatibility path; strong applications must refuse it.
    $LoginPage = (& curl.exe --fail --silent --show-error -b $CookieJar -c $CookieJar `
        -H "Host: identity.mattediworks.com" "http://127.0.0.1:$Port/login") -join "`n"
    $LoginCSRF = [regex]::Match($LoginPage, 'name="csrf_token" value="([^"]+)"').Groups[1].Value
    if ([string]::IsNullOrWhiteSpace($LoginCSRF)) { throw "Identity login CSRF missing" }
    $LoginStatus = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" `
        -b $CookieJar -c $CookieJar -H "Host: identity.mattediworks.com" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "email=$Email" --data-urlencode "password=$Password" `
        --data-urlencode "csrf_token=$LoginCSRF" --data-urlencode "return_to=/account" `
        "http://127.0.0.1:$Port/login"
    if ([int]$LoginStatus -ne 303) { throw "Identity password login failed: $LoginStatus" }
    $AccountStatus = Invoke-IdentityStatus "GET" "/account" $CookieJar
    if ($AccountStatus -ne 200) { throw "password Identity session was not established" }
    Write-Host "PASS legacy password session remains available for standard policy"

    $StandardPath = New-AuthorizePath $StandardClientID $StandardRedirect
    $Standard = Request-Redirect "identity.mattediworks.com" $StandardPath $CookieJar $Headers
    if ($Standard.Status -ne 302 -or $Standard.Location -notlike "$StandardRedirect*") {
        throw "standard application rejected compatible password session: status=$($Standard.Status) location=$($Standard.Location)"
    }
    if ($Standard.Location -notmatch '[?&]code=mwac_') {
        throw "standard authorization code missing: location=$($Standard.Location)"
    }
    Write-Host "PASS standard application accepts configured legacy assurance"

    $StrongPath = New-AuthorizePath $StrongClientID $StrongRedirect
    $Strong = Request-Redirect "identity.mattediworks.com" $StrongPath $CookieJar $Headers
    if ($Strong.Status -ne 303 -or $Strong.Location -notmatch 'auth=strong') {
        throw "strong application did not require strong authentication: status=$($Strong.Status) location=$($Strong.Location)"
    }
    Write-Host "PASS strong application rejects password-only assurance"

    $ACRExtra = "acr_values=$([uri]::EscapeDataString('urn:mw:webauthn'))&max_age=300"
    $StepUpPath = New-AuthorizePath $StandardClientID $StandardRedirect $ACRExtra
    $StepUp = Request-Redirect "identity.mattediworks.com" $StepUpPath $CookieJar $Headers
    if ($StepUp.Status -ne 303 -or $StepUp.Location -notmatch 'auth=webauthn') {
        throw "explicit WebAuthn ACR did not force step-up: status=$($StepUp.Status) location=$($StepUp.Location)"
    }
    Write-Host "PASS explicit WebAuthn ACR + max_age forces step-up"

    Write-Host "== Step 12 operator recovery =="
    $RecoveryOutput = (& docker @ComposeArgs run --rm --entrypoint /app/recover-identity mw-identity --email $Email) -join "`n"
    if ($LASTEXITCODE -ne 0) { throw "operator recovery start failed" }
    $RecoveryToken = [regex]::Match($RecoveryOutput, 'mwrec_[A-Za-z0-9_-]+').Value
    if ([string]::IsNullOrWhiteSpace($RecoveryToken)) { throw "recovery capability missing from CLI output" }

    $Revoked = Request-Redirect "identity.mattediworks.com" "/account" $CookieJar $Headers
    if ($Revoked.Status -ne 303 -or $Revoked.Location -notmatch '^/login') {
        throw "operator recovery did not revoke the existing Identity session"
    }
    Write-Host "PASS operator recovery immediately revokes existing Identity sessions"

    # Password login is also blocked while recovery_pending is set.
    $RecoveryLoginPage = (& curl.exe --fail --silent --show-error -b $RecoveryCookieJar -c $RecoveryCookieJar `
        -H "Host: identity.mattediworks.com" "http://127.0.0.1:$Port/login") -join "`n"
    $RecoveryCSRF = [regex]::Match($RecoveryLoginPage, 'name="csrf_token" value="([^"]+)"').Groups[1].Value
    $PendingLogin = & curl.exe --silent --show-error --output NUL --write-out "%{http_code}" `
        -b $RecoveryCookieJar -c $RecoveryCookieJar -H "Host: identity.mattediworks.com" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "email=$Email" --data-urlencode "password=$Password" `
        --data-urlencode "csrf_token=$RecoveryCSRF" --data-urlencode "return_to=/account" `
        "http://127.0.0.1:$Port/login"
    if ([int]$PendingLogin -ne 401) { throw "recovery_pending password login was not blocked: $PendingLogin" }
    Write-Host "PASS recovery_pending blocks normal password login"

    $RecoveryOptions = Invoke-IdentityStatus "POST" "/api/recovery/webauthn/options" "" @{ "X-MW-Recovery" = $RecoveryToken }
    if ($RecoveryOptions -ne 200) { throw "valid recovery capability could not start replacement passkey ceremony: $RecoveryOptions" }
    Write-Host "PASS recovery capability starts replacement WebAuthn ceremony"

    $NoEmailRecovery = Invoke-IdentityStatus "POST" "/api/recovery/email"
    if ($NoEmailRecovery -ne 404) { throw "unexpected email recovery route exists: $NoEmailRecovery" }
    Write-Host "PASS no email recovery route exists"

    & docker @ComposeArgs run --rm --entrypoint /app/recover-identity mw-identity --cancel --email $Email | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "operator recovery cancel failed" }
    $CancelledCapability = Invoke-IdentityStatus "POST" "/api/recovery/webauthn/options" "" @{ "X-MW-Recovery" = $RecoveryToken }
    if ($CancelledCapability -ne 401) { throw "cancelled recovery capability remained usable: $CancelledCapability" }
    Write-Host "PASS recovery cancellation invalidates capability and does not restore old sessions"

    Write-Host "STEP 12 API PASS"
    Write-Host "Browser checkpoint user: $Email"
    Write-Host "Browser checkpoint password: $Password"
    Write-Host "Browser Identity: http://localhost:7100/login"
    Write-Host "Browser BFF: http://localhost:7200/auth/login?return_to=%2F"
    Write-Host "Final STEP 12 E2E PASS requires the real browser/passkey checkpoint in docs/step-12-strong-auth-totp-recovery.md."
}
finally {
    foreach ($Path in @($CookieJar, $RecoveryCookieJar, $Headers, "$Headers.body")) {
        if (-not [string]::IsNullOrWhiteSpace($Path)) {
            Remove-Item -LiteralPath $Path -ErrorAction SilentlyContinue
        }
    }
    Pop-Location
}
