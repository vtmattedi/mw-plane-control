param(
    [int]$Port = 3050,
    [int]$RPPort = 7350
)

$ErrorActionPreference = "Stop"
$env:GATEWAY_HOST_PORT = "$Port"

$Root = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot ".."))
$Step7Env = Join-Path $Root ".env.step7.local"
$Step8Env = Join-Path $Root ".env.step8.local"

if (-not (Test-Path $Step7Env)) {
    throw ".env.step7.local is required from the passing Step 07 environment."
}

function Set-EnvValue([System.Collections.Generic.List[string]]$Lines, [string]$Name, [string]$Value) {
    $Prefix = "$Name="
    for ($i = 0; $i -lt $Lines.Count; $i++) {
        if ($Lines[$i].StartsWith($Prefix)) {
            $Lines[$i] = "$Name=$Value"
            return
        }
    }
    $Lines.Add("$Name=$Value")
}

$EnvLines = [System.Collections.Generic.List[string]]::new()
foreach ($Line in (Get-Content $Step7Env)) { $EnvLines.Add($Line) }
# Step 07 browser testing may have temporarily used localhost. Step 08 returns
# the OIDC issuer/topology to the canonical values used by BFF/Identity.
Set-EnvValue $EnvLines "IDENTITY_PUBLIC_ISSUER" "https://identity.mattediworks.com"
Set-EnvValue $EnvLines "IDENTITY_WEBAUTHN_RP_ID" "identity.mattediworks.com"
Set-EnvValue $EnvLines "IDENTITY_WEBAUTHN_ORIGINS" "https://identity.mattediworks.com"
Set-EnvValue $EnvLines "GATEWAY_HOST_PORT" "$Port"
$EnvLines | Set-Content -Path $Step8Env -Encoding ascii

$GatewayBase = "http://127.0.0.1:$Port"
$RPBase = "http://localhost:$RPPort"
$AdminHost = "admin.mattediworks.com"
$IdentityHost = "identity.mattediworks.com"
$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()

$IdentityEmail = "step08-$Stamp@mattediworks.test"
$IdentityPassword = "Step08-Identity-Password-$Stamp"
$PrimaryEmail = $IdentityEmail
$PrimaryPassword = "Step08-Local-Primary-$Stamp"
$SecondaryEmail = "step08-secondary-$Stamp@mattediworks.test"
$SecondaryPassword = "Step08-Local-Secondary-$Stamp"

$AdminJar = Join-Path $env:TEMP "mw-step08-admin-$Stamp.cookies"
$RPPrimaryJar = Join-Path $env:TEMP "mw-step08-rp-primary-$Stamp.cookies"
$RPSigninJar = Join-Path $env:TEMP "mw-step08-rp-signin-$Stamp.cookies"
$RPSecondaryJar = Join-Path $env:TEMP "mw-step08-rp-secondary-$Stamp.cookies"
$RPPendingJar = Join-Path $env:TEMP "mw-step08-rp-pending-$Stamp.cookies"
$RPDenyJar = Join-Path $env:TEMP "mw-step08-rp-deny-$Stamp.cookies"
$Headers = Join-Path $env:TEMP "mw-step08-$Stamp.headers"
$Body = Join-Path $env:TEMP "mw-step08-$Stamp.body"
$RPExe = Join-Path $env:TEMP "mw-step08-link-rp-$Stamp.exe"
$RPStdout = Join-Path $env:TEMP "mw-step08-link-rp-$Stamp.stdout.log"
$RPStderr = Join-Path $env:TEMP "mw-step08-link-rp-$Stamp.stderr.log"
$RPProcess = $null

function Invoke-HTTP(
    [string]$Method,
    [string]$URL,
    [string]$CookieJar,
    [string]$HostName = "",
    [hashtable]$Form = $null,
    [object]$JSONBody = $null,
    [hashtable]$HeadersExtra = $null
) {
    Remove-Item $Headers -ErrorAction SilentlyContinue
    Remove-Item $Body -ErrorAction SilentlyContinue

    $Args = @(
        "--silent",
        "--show-error",
        "--dump-header", $Headers,
        "--output", $Body,
        "--write-out", "%{http_code}",
        "-b", $CookieJar,
        "-c", $CookieJar
    )

    if ($Method -ne "GET") {
        $Args += @("-X", $Method)
    }
    if (-not [string]::IsNullOrWhiteSpace($HostName)) {
        $Args += @("-H", "Host: $HostName")
    }
    if ($null -ne $HeadersExtra) {
        foreach ($Entry in $HeadersExtra.GetEnumerator()) {
            $Args += @("-H", "$($Entry.Key): $($Entry.Value)")
        }
    }
    if ($null -ne $Form) {
        $Args += @("-H", "Content-Type: application/x-www-form-urlencoded")
        foreach ($Entry in $Form.GetEnumerator()) {
            $Args += @("--data-urlencode", "$($Entry.Key)=$($Entry.Value)")
        }
    }
    if ($null -ne $JSONBody) {
        $Encoded = $JSONBody | ConvertTo-Json -Depth 10 -Compress
        $Args += @("-H", "Content-Type: application/json", "--data-raw", $Encoded)
    }
    $Args += $URL

    $StatusRaw = (& curl.exe @Args) -join ""
    if ($LASTEXITCODE -ne 0) {
        throw "curl failed: $Method $URL"
    }
    $BodyText = ""
    if (Test-Path $Body) {
        $BodyText = Get-Content $Body -Raw
    }
    $Location = ""
    if (Test-Path $Headers) {
        $Line = Get-Content $Headers |
            Where-Object { $_ -match '^Location:\s*(.+?)\s*$' } |
            Select-Object -First 1
        if ($null -ne $Line) {
            $Location = [regex]::Match($Line, '^Location:\s*(.+?)\s*$').Groups[1].Value
        }
    }
    return @{
        Status = [int]$StatusRaw
        BodyText = $BodyText
        Location = $Location
    }
}

function Parse-JSON([hashtable]$Response) {
    if ([string]::IsNullOrWhiteSpace($Response.BodyText)) { return $null }
    return $Response.BodyText | ConvertFrom-Json
}

function Path-And-Query([string]$Value) {
    if ($Value -match '^https?://') {
        return ([System.Uri]$Value).PathAndQuery
    }
    return $Value
}

function Invoke-AdminJSON(
    [string]$Method,
    [string]$Path,
    [object]$Payload = $null,
    [string]$CSRF = ""
) {
    $Extra = @{ Accept = "application/json" }
    if (-not [string]::IsNullOrWhiteSpace($CSRF)) {
        $Extra["X-MW-CSRF"] = $CSRF
    }
    $Response = Invoke-HTTP $Method "$GatewayBase$Path" $AdminJar $AdminHost $null $Payload $Extra
    return @{
        Status = $Response.Status
        Body = Parse-JSON $Response
        BodyText = $Response.BodyText
        Location = $Response.Location
    }
}

function Invoke-IdentityAuthorize([string]$AuthorizeURL) {
    $Path = Path-And-Query $AuthorizeURL
    return Invoke-HTTP "GET" "$GatewayBase$Path" $AdminJar $IdentityHost
}

function Complete-RP-OIDC([string]$StartPath, [string]$RPJar) {
    $Start = Invoke-HTTP "GET" "$RPBase$StartPath" $RPJar
    if ($Start.Status -ne 302 -or [string]::IsNullOrWhiteSpace($Start.Location)) {
        throw "RP did not start OIDC: $StartPath ($($Start.Status))"
    }
    $Authorization = Invoke-IdentityAuthorize $Start.Location
    if (($Authorization.Status -ne 302 -and $Authorization.Status -ne 303) -or
        [string]::IsNullOrWhiteSpace($Authorization.Location)) {
        throw "Identity did not issue OIDC callback: status=$($Authorization.Status) body=$($Authorization.BodyText)"
    }
    if ($Authorization.Location -notmatch '[?&]code=') {
        throw "Identity callback did not contain an authorization code: $($Authorization.Location)"
    }
    return Invoke-HTTP "GET" $Authorization.Location $RPJar
}

function Assert-AuthorizationDenied([string]$StartPath, [string]$RPJar, [string]$Label) {
    $Start = Invoke-HTTP "GET" "$RPBase$StartPath" $RPJar
    if ($Start.Status -ne 302) {
        throw "${Label}: RP start failed with $($Start.Status)"
    }
    $Authorization = Invoke-IdentityAuthorize $Start.Location
    if ($Authorization.Location -match '[?&]code=') {
        throw "${Label}: Identity issued an authorization code"
    }
    $ErrorRedirect = $Authorization.Location -match '[?&]error='
    if ($Authorization.Status -lt 400 -and -not $ErrorRedirect) {
        throw "${Label}: expected denial; status=$($Authorization.Status) location=$($Authorization.Location) body=$($Authorization.BodyText)"
    }
    Write-Host "PASS $Label"
}

try {
    Write-Host "== Step 08 static sanity =="
    & "$PSScriptRoot\sanity-step-08.ps1"
    if ($LASTEXITCODE -ne 0) { throw "Step 08 static sanity failed" }

    Write-Host "== Start Step 08 control plane =="
    Push-Location $Root
    try {
        docker compose --env-file $Step8Env up -d --build
        if ($LASTEXITCODE -ne 0) { throw "docker compose up failed" }
    } finally {
        Pop-Location
    }

    $Healthy = $false
    for ($i = 0; $i -lt 45; $i++) {
        $Health = Invoke-HTTP "GET" "$GatewayBase/health" $AdminJar $IdentityHost
        if ($Health.Status -eq 200) { $Healthy = $true; break }
        Start-Sleep -Seconds 1
    }
    if (-not $Healthy) {
        Push-Location $Root
        try {
            docker compose --env-file $Step8Env ps
            docker compose --env-file $Step8Env logs --tail=180
        } finally { Pop-Location }
        throw "Identity did not become healthy"
    }

    Write-Host "== Create Step 08 Identity principal =="
    Push-Location $Root
    try {
        $IdentityPassword | docker compose --env-file $Step8Env exec -T mw-identity `
            /app/create-superadmin `
            --email $IdentityEmail `
            --display-name "Step 08 Identity" `
            --password-stdin
        if ($LASTEXITCODE -ne 0) { throw "create-superadmin failed" }
    } finally { Pop-Location }

    Write-Host "== Establish privileged BFF session =="
    $BFFLogin = Invoke-HTTP "GET" "$GatewayBase/api/auth/login?return_to=%2Fidentity" $AdminJar $AdminHost
    if ($BFFLogin.Status -ne 302) { throw "BFF login did not redirect" }
    $IdentityAuth = Invoke-HTTP "GET" "$GatewayBase$(Path-And-Query $BFFLogin.Location)" $AdminJar $IdentityHost
    if ($IdentityAuth.Status -ne 303) { throw "Identity authorization did not enter login" }
    $LoginPage = Invoke-HTTP "GET" "$GatewayBase$($IdentityAuth.Location)" $AdminJar $IdentityHost
    if ($LoginPage.Status -ne 200) { throw "Identity login page failed" }
    $CSRF = [regex]::Match($LoginPage.BodyText, 'name="csrf_token" value="([^"]+)"').Groups[1].Value
    $ReturnToEncoded = [regex]::Match($LoginPage.BodyText, 'name="return_to" value="([^"]+)"').Groups[1].Value
    $ReturnTo = [System.Net.WebUtility]::HtmlDecode($ReturnToEncoded)
    if ([string]::IsNullOrWhiteSpace($CSRF) -or [string]::IsNullOrWhiteSpace($ReturnTo)) {
        throw "Identity login form tokens missing"
    }
    $Login = Invoke-HTTP "POST" "$GatewayBase/login" $AdminJar $IdentityHost @{
        email = $IdentityEmail
        password = $IdentityPassword
        csrf_token = $CSRF
        return_to = $ReturnTo
    }
    if ($Login.Status -ne 303) { throw "Identity password login failed" }
    $IdentityComplete = Invoke-HTTP "GET" "$GatewayBase$(Path-And-Query $BFFLogin.Location)" $AdminJar $IdentityHost
    if (($IdentityComplete.Status -ne 302 -and $IdentityComplete.Status -ne 303) -or [string]::IsNullOrWhiteSpace($IdentityComplete.Location)) {
        throw "Identity did not complete BFF authorization"
    }
    $BFFCallback = Invoke-HTTP "GET" "$GatewayBase$(Path-And-Query $IdentityComplete.Location)" $AdminJar $AdminHost
    if ($BFFCallback.Status -ne 303) { throw "BFF callback failed" }

    $CSRFResult = Invoke-AdminJSON "POST" "/api/auth/csrf"
    if ($CSRFResult.Status -ne 200) { throw "BFF CSRF rotation failed" }
    $AdminCSRF = $CSRFResult.Body.csrf_token

    $Users = Invoke-AdminJSON "GET" "/api/identity/users"
    if ($Users.Status -ne 200) { throw "Identity users listing failed" }
    $IdentityUser = $Users.Body.users | Where-Object { $_.email -eq $IdentityEmail } | Select-Object -First 1
    if ($null -eq $IdentityUser) { throw "Step 08 Identity user missing" }
    Write-Host "PASS privileged Admin/Identity session"

    Write-Host "== Register reference relying application =="
    $ClientID = "step08_link_rp_$Stamp"
    $Created = Invoke-AdminJSON "POST" "/api/identity/applications" @{
        name = "Step 08 Account Linking RP"
        client_id = $ClientID
        application_type = "confidential"
        authentication_policy = "standard"
        redirect_uris = @("$RPBase/oidc/callback")
        post_logout_redirect_uris = @()
        scopes = @("openid", "profile", "email")
        claims = @()
        audiences = @()
    } $AdminCSRF
    if ($Created.Status -ne 201) { throw "reference RP registration failed: $($Created.BodyText)" }
    $AppID = $Created.Body.application.id
    $ClientSecret = $Created.Body.client_secret
    if ([string]::IsNullOrWhiteSpace($AppID) -or [string]::IsNullOrWhiteSpace($ClientSecret)) {
        throw "reference RP id/secret missing"
    }

    $Assigned = Invoke-AdminJSON "PUT" "/api/identity/applications/$AppID/assignments/$($IdentityUser.id)" @{ role = "user" } $AdminCSRF
    if ($Assigned.Status -ne 200) { throw "reference RP assignment failed" }
    Write-Host "PASS ordinary application assignment"

    Write-Host "== Build/start disposable reference RP =="
    $RPDir = Join-Path $Root "testapps\account-link-rp"
    Push-Location $RPDir
    try {
        go build -o $RPExe .
        if ($LASTEXITCODE -ne 0) { throw "reference RP build failed" }
    } finally { Pop-Location }

    $env:LINK_RP_ADDR = "127.0.0.1:$RPPort"
    $env:LINK_RP_PUBLIC_BASE_URL = $RPBase
    $env:LINK_RP_IDENTITY_ISSUER = "https://identity.mattediworks.com"
    $env:LINK_RP_IDENTITY_BROWSER_BASE = $GatewayBase
    $env:LINK_RP_IDENTITY_INTERNAL_BASE = $GatewayBase
    $env:LINK_RP_IDENTITY_HOST = $IdentityHost
    $env:LINK_RP_CLIENT_ID = $ClientID
    $env:LINK_RP_CLIENT_SECRET = $ClientSecret
    $env:LINK_RP_COOKIE_SECURE = "false"
    $env:LINK_RP_TEST_MODE = "true"
    $env:LINK_RP_USER1_ID = "local_primary"
    $env:LINK_RP_USER1_EMAIL = $PrimaryEmail
    $env:LINK_RP_USER1_PASSWORD = $PrimaryPassword
    $env:LINK_RP_USER2_ID = "local_secondary"
    $env:LINK_RP_USER2_EMAIL = $SecondaryEmail
    $env:LINK_RP_USER2_PASSWORD = $SecondaryPassword

    $RPProcess = Start-Process -FilePath $RPExe -PassThru -RedirectStandardOutput $RPStdout -RedirectStandardError $RPStderr
    $RPHealthy = $false
    for ($i = 0; $i -lt 30; $i++) {
        if ($RPProcess.HasExited) { break }
        $Health = Invoke-HTTP "GET" "$RPBase/health" $RPPrimaryJar
        if ($Health.Status -eq 200) { $RPHealthy = $true; break }
        Start-Sleep -Milliseconds 300
    }
    if (-not $RPHealthy) {
        if (Test-Path $RPStdout) { Get-Content $RPStdout }
        if (Test-Path $RPStderr) { Get-Content $RPStderr }
        throw "reference RP did not become healthy"
    }

    Write-Host "== Link from existing authenticated local account =="
    $LocalLogin = Invoke-HTTP "POST" "$RPBase/local/login" $RPPrimaryJar "" @{
        email = $PrimaryEmail
        password = $PrimaryPassword
    }
    if ($LocalLogin.Status -ne 204) { throw "primary local login failed" }
    $LinkedCallback = Complete-RP-OIDC "/link/start" $RPPrimaryJar
    if ($LinkedCallback.Status -ne 303 -or $LinkedCallback.Location -ne "/me") {
        throw "link callback failed: $($LinkedCallback.Status) $($LinkedCallback.BodyText)"
    }
    $Me = Invoke-HTTP "GET" "$RPBase/me" $RPPrimaryJar
    $MeBody = Parse-JSON $Me
    if ($Me.Status -ne 200 -or $MeBody.local_user.id -ne "local_primary" -or $MeBody.external_identities.Count -ne 1) {
        throw "linked local account state invalid"
    }
    $LinkedSubject = $MeBody.external_identities[0].subject
    if ([string]::IsNullOrWhiteSpace($LinkedSubject)) { throw "linked subject missing" }
    Write-Host "PASS existing account -> validated (issuer, sub) mapping"

    Write-Host "== Sign in through existing link =="
    $LocalLogout = Invoke-HTTP "POST" "$RPBase/local/logout" $RPPrimaryJar
    if ($LocalLogout.Status -ne 204) { throw "local logout failed" }
    $SigninCallback = Complete-RP-OIDC "/auth/mw/start" $RPSigninJar
    if ($SigninCallback.Status -ne 303 -or $SigninCallback.Location -ne "/me") {
        throw "linked Identity sign-in did not reach /me"
    }
    $SigninMe = Invoke-HTTP "GET" "$RPBase/me" $RPSigninJar
    $SigninBody = Parse-JSON $SigninMe
    if ($SigninMe.Status -ne 200 -or $SigninBody.local_user.id -ne "local_primary" -or $SigninBody.auth_method -ne "mw_identity") {
        throw "linked Identity sign-in mapped to wrong local account"
    }
    Write-Host "PASS Sign in with MW Identity -> linked local user"

    Write-Host "== Duplicate subject protection =="
    $SecondLogin = Invoke-HTTP "POST" "$RPBase/local/login" $RPSecondaryJar "" @{
        email = $SecondaryEmail
        password = $SecondaryPassword
    }
    if ($SecondLogin.Status -ne 204) { throw "secondary local login failed" }
    $DuplicateCallback = Complete-RP-OIDC "/link/start" $RPSecondaryJar
    if ($DuplicateCallback.Status -ne 409) {
        throw "duplicate subject was not rejected: $($DuplicateCallback.Status) $($DuplicateCallback.BodyText)"
    }
    Write-Host "PASS duplicate (issuer, sub) cannot move to another local user"

    Write-Host "== Safe unlink =="
    $WrongUnlink = Invoke-HTTP "POST" "$RPBase/unlink" $RPSigninJar "" @{ password = "wrong-$PrimaryPassword" }
    if ($WrongUnlink.Status -ne 403) { throw "unlink did not require fresh local authentication" }
    $Unlink = Invoke-HTTP "POST" "$RPBase/unlink" $RPSigninJar "" @{ password = $PrimaryPassword }
    if ($Unlink.Status -ne 204) { throw "safe unlink failed: $($Unlink.BodyText)" }
    $AfterUnlink = Invoke-HTTP "GET" "$RPBase/me" $RPSigninJar
    if ($AfterUnlink.Status -ne 401) { throw "unlink did not revoke local sessions" }
    Write-Host "PASS fresh local auth + unlink + session revocation"

    Write-Host "== No email auto-link on first Identity sign-in =="
    $PendingCallback = Complete-RP-OIDC "/auth/mw/start" $RPPendingJar
    if ($PendingCallback.Status -ne 303 -or $PendingCallback.Location -ne "/link-required") {
        throw "unlinked Identity was auto-linked or pending-link flow missing"
    }
    $Required = Invoke-HTTP "GET" "$RPBase/link-required" $RPPendingJar
    $RequiredBody = Parse-JSON $Required
    if ($Required.Status -ne 200 -or $RequiredBody.requires_existing_auth -ne $true -or $RequiredBody.identity_email -ne $PrimaryEmail) {
        throw "same-email pending-link proof failed"
    }
    $WrongExisting = Invoke-HTTP "POST" "$RPBase/link-existing" $RPPendingJar "" @{
        email = $PrimaryEmail
        password = "wrong-$PrimaryPassword"
    }
    if ($WrongExisting.Status -ne 401) { throw "pending link did not require valid existing-account authentication" }
    $StillPending = Invoke-HTTP "GET" "$RPBase/link-required" $RPPendingJar
    if ($StillPending.Status -ne 200) { throw "failed local auth consumed pending-link capability" }
    $Existing = Invoke-HTTP "POST" "$RPBase/link-existing" $RPPendingJar "" @{
        email = $PrimaryEmail
        password = $PrimaryPassword
    }
    if ($Existing.Status -ne 303 -or $Existing.Location -ne "/me") { throw "existing-account link completion failed" }
    $ExistingMe = Invoke-HTTP "GET" "$RPBase/me" $RPPendingJar
    $ExistingBody = Parse-JSON $ExistingMe
    if ($ExistingMe.Status -ne 200 -or $ExistingBody.local_user.id -ne "local_primary" -or $ExistingBody.auth_method -ne "mw_identity_linked_existing") {
        throw "existing-account authentication linked wrong local user"
    }
    if ($ExistingBody.external_identities[0].subject -ne $LinkedSubject) {
        throw "re-link changed stable Identity subject"
    }
    Write-Host "PASS same email alone never links; existing local auth is required"

    Write-Host "== Assignment denial =="
    $Removed = Invoke-AdminJSON "DELETE" "/api/identity/applications/$AppID/assignments/$($IdentityUser.id)" $null $AdminCSRF
    if ($Removed.Status -ne 204) { throw "assignment removal failed" }
    Assert-AuthorizationDenied "/auth/mw/start" $RPDenyJar "removed assignment denies authorization"

    $Restored = Invoke-AdminJSON "PUT" "/api/identity/applications/$AppID/assignments/$($IdentityUser.id)" @{ role = "user" } $AdminCSRF
    if ($Restored.Status -ne 200) { throw "assignment restore failed" }

    Write-Host "== Disabled application denial =="
    $Disabled = Invoke-AdminJSON "POST" "/api/identity/applications/$AppID/disable" $null $AdminCSRF
    if ($Disabled.Status -ne 200 -or $Disabled.Body.enabled -ne $false) { throw "application disable failed" }
    Assert-AuthorizationDenied "/auth/mw/start" $RPDenyJar "disabled application denies authorization"
    $Enabled = Invoke-AdminJSON "POST" "/api/identity/applications/$AppID/enable" $null $AdminCSRF
    if ($Enabled.Status -ne 200 -or $Enabled.Body.enabled -ne $true) { throw "application re-enable failed" }

    $Audit = Invoke-HTTP "GET" "$RPBase/test/audit" $RPPrimaryJar
    $AuditBody = Parse-JSON $Audit
    if ($Audit.Status -ne 200) { throw "reference RP audit endpoint failed" }
    $Events = @($AuditBody.audit | ForEach-Object { $_.type })
    foreach ($RequiredEvent in @(
        "link.completed",
        "signin.completed",
        "link.duplicate_rejected",
        "unlink.completed",
        "link.authentication_required",
        "link.existing_completed"
    )) {
        if ($Events -notcontains $RequiredEvent) { throw "reference RP audit missing $RequiredEvent" }
    }
    Write-Host "PASS linking audit coverage"

    Write-Host ""
    Write-Host "STEP 08 E2E PASS"
}
finally {
    if ($null -ne $RPProcess -and -not $RPProcess.HasExited) {
        Stop-Process -Id $RPProcess.Id -Force -ErrorAction SilentlyContinue
    }
    foreach ($Path in @(
        $AdminJar, $RPPrimaryJar, $RPSigninJar, $RPSecondaryJar,
        $RPPendingJar, $RPDenyJar, $Headers, $Body, $RPExe,
        $RPStdout, $RPStderr
    )) {
        Remove-Item $Path -Force -ErrorAction SilentlyContinue
    }
}
