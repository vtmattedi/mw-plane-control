param(
    [int]$Port = 3050,
    [Parameter(Mandatory = $true)]
    [string]$EnvFile
)

$ErrorActionPreference = "Stop"

$Base = "http://127.0.0.1:$Port"
$AdminHost = "admin.mattediworks.com"
$IdentityHost = "identity.mattediworks.com"
$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$Email = "step06-$Stamp@mattediworks.test"
$Password = "Step06-Integration-Password-$Stamp"

$CookieJar = Join-Path $env:TEMP "mw-step06-$Stamp.cookies"
$Headers = Join-Path $env:TEMP "mw-step06-$Stamp.headers"

function Request-Redirect(
    [string]$HostName,
    [string]$Path
) {
    $Status = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --dump-header $Headers `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        "$Base$Path"

    if ($LASTEXITCODE -ne 0) {
        throw "request failed: $HostName$Path"
    }

    $Line = Get-Content $Headers |
        Where-Object { $_ -match '^Location:\s*(.+)\s*$' } |
        Select-Object -First 1

    if ($null -eq $Line) {
        throw "Location missing: $HostName$Path"
    }

    return @{
        Status = $Status
        Location = [regex]::Match(
            $Line,
            '^Location:\s*(.+?)\s*$'
        ).Groups[1].Value
    }
}

function Path-And-Query([string]$URL) {
    return ([System.Uri]$URL).PathAndQuery
}

function Invoke-AdminJSON(
    [string]$Method,
    [string]$Path,
    [object]$Body = $null,
    [string]$CSRF = ""
) {
    $Args = @(
        "--silent",
        "--show-error",
        "-b", $CookieJar,
        "-c", $CookieJar,
        "-H", "Host: $AdminHost",
        "-H", "Accept: application/json"
    )

    if ($Method -ne "GET") {
        $Args += @("-X", $Method)
    }

    if (-not [string]::IsNullOrWhiteSpace($CSRF)) {
        $Args += @("-H", "X-MW-CSRF: $CSRF")
    }

    if ($null -ne $Body) {
        $JSON = $Body | ConvertTo-Json -Depth 10 -Compress
        $Args += @(
            "-H", "Content-Type: application/json",
            "--data-raw", $JSON
        )
    }

    $Args += @(
        "--write-out", "`n%{http_code}",
        "$Base$Path"
    )

    $Raw = (& curl.exe @Args) -join "`n"
    if ($LASTEXITCODE -ne 0) {
        throw "curl failed: $Method $Path"
    }

    $LastNewline = $Raw.LastIndexOf("`n")
    if ($LastNewline -lt 0) {
        throw "response status separator missing"
    }

    $BodyText = $Raw.Substring(0, $LastNewline)
    $Status = [int]$Raw.Substring($LastNewline + 1)

    $Parsed = $null
    if (-not [string]::IsNullOrWhiteSpace($BodyText)) {
        $Parsed = $BodyText | ConvertFrom-Json
    }

    return @{
        Status = $Status
        Body = $Parsed
        RawBody = $BodyText
    }
}

try {
    Write-Host "== Step 06 Identity Admin through BFF =="

    $Password | docker compose --env-file $EnvFile exec -T mw-identity `
        /app/create-superadmin `
        --email $Email `
        --display-name "Step 06 Root" `
        --password-stdin

    if ($LASTEXITCODE -ne 0) {
        throw "create-superadmin failed"
    }

    # Full BFF -> Identity login.
    $BFFLogin = Request-Redirect `
        $AdminHost `
        "/api/auth/login?return_to=%2Fidentity"

    if ($BFFLogin.Status -ne "302") {
        throw "BFF login did not redirect"
    }

    $AuthorizePath = Path-And-Query $BFFLogin.Location

    $IdentityAuth = Request-Redirect `
        $IdentityHost `
        $AuthorizePath

    if ($IdentityAuth.Status -ne "303") {
        throw "Identity authorization did not enter login"
    }

    $LoginPage = (& curl.exe `
        --fail `
        --silent `
        --show-error `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $IdentityHost" `
        "$Base$($IdentityAuth.Location)") -join "`n"

    $CSRF = [regex]::Match(
        $LoginPage,
        'name="csrf_token" value="([^"]+)"'
    ).Groups[1].Value

    $ReturnToEncoded = [regex]::Match(
        $LoginPage,
        'name="return_to" value="([^"]+)"'
    ).Groups[1].Value

    $ReturnTo = [System.Net.WebUtility]::HtmlDecode(
        $ReturnToEncoded
    )

    $LoginStatus = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $IdentityHost" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "email=$Email" `
        --data-urlencode "password=$Password" `
        --data-urlencode "csrf_token=$CSRF" `
        --data-urlencode "return_to=$ReturnTo" `
        "$Base/login"

    if ($LoginStatus -ne "303") {
        throw "Identity login failed"
    }

    $IdentityComplete = Request-Redirect `
        $IdentityHost `
        $AuthorizePath

    $CallbackPath = Path-And-Query $IdentityComplete.Location

    $BFFCallback = Request-Redirect `
        $AdminHost `
        $CallbackPath

    if ($BFFCallback.Status -ne "303") {
        throw "BFF callback failed"
    }

    Write-Host "PASS privileged Identity login -> BFF session"

    $CSRFResult = Invoke-AdminJSON `
        "POST" `
        "/api/auth/csrf"

    if ($CSRFResult.Status -ne 200) {
        throw "BFF CSRF rotation failed"
    }

    $AdminCSRF = $CSRFResult.Body.csrf_token

    $Who = Invoke-AdminJSON `
        "GET" `
        "/api/identity/whoami"

    if ($Who.Status -ne 200 -or
        $Who.Body.role -ne "superadmin") {
        throw "Identity admin role/permission check failed"
    }

    if ($Who.Body.permissions -notcontains
        "identity.assignments.write") {
        throw "superadmin assignment permission missing"
    }

    Write-Host "PASS Identity-admin audience/scope/permissions"

    $Users = Invoke-AdminJSON `
        "GET" `
        "/api/identity/users"

    if ($Users.Status -ne 200) {
        throw "Identity user listing failed"
    }

    $CurrentUser = $Users.Body.users |
        Where-Object { $_.email -eq $Email } |
        Select-Object -First 1

    if ($null -eq $CurrentUser) {
        throw "current Identity user not found in admin listing"
    }

    Write-Host "PASS list Identity users"

    $TestClientID = "step06_managed_$Stamp"

    $CreateBody = @{
        name = "Step 06 Managed App"
        client_id = $TestClientID
        application_type = "confidential"
        authentication_policy = "standard"
        redirect_uris = @(
            "https://step06.example/initial"
        )
        post_logout_redirect_uris = @()
        delete_callback_uri = "https://step06.example/account-deleted"
        scopes = @("openid", "profile", "email")
        claims = @()
        audiences = @()
    }

    $NoCSRF = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications" `
        $CreateBody

    if ($NoCSRF.Status -ne 403) {
        throw "Identity mutation without BFF CSRF was not rejected"
    }

    Write-Host "PASS BFF CSRF protects Identity mutations"

    $NoCallbackBody = $CreateBody.Clone()
    $NoCallbackBody.client_id = "${TestClientID}_nocallback"
    $NoCallbackBody.Remove("delete_callback_uri")
    $NoCallback = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications" `
        $NoCallbackBody `
        $AdminCSRF

    if ($NoCallback.Status -ne 400 -or $NoCallback.RawBody -notmatch "delete_callback_required") {
        throw "application without a delete callback was not rejected: $($NoCallback.Status) $($NoCallback.RawBody)"
    }

    Write-Host "PASS user-facing applications require a delete callback"

    $Created = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications" `
        $CreateBody `
        $AdminCSRF

    if ($Created.Status -ne 201) {
        throw "create Identity application failed: $($Created.RawBody)"
    }

    $AppID = $Created.Body.application.id
    $InitialSecret = $Created.Body.client_secret

    if ([string]::IsNullOrWhiteSpace($AppID) -or
        [string]::IsNullOrWhiteSpace($InitialSecret)) {
        throw "created application id/secret missing"
    }

    Write-Host "PASS create OIDC application"

    $UpdateBody = @{
        name = "Step 06 Managed App Updated"
        authentication_policy = "standard"
        redirect_uris = @(
            "https://step06.example/exact-callback"
        )
        post_logout_redirect_uris = @()
        delete_callback_uri = "https://step06.example/account-deleted"
        scopes = @("openid", "profile", "email")
        claims = @()
        audiences = @()
    }

    $Updated = Invoke-AdminJSON `
        "PATCH" `
        "/api/identity/applications/$AppID" `
        $UpdateBody `
        $AdminCSRF

    if ($Updated.Status -ne 200) {
        throw "update application failed: $($Updated.RawBody)"
    }

    if ($Updated.Body.redirect_uris.Count -ne 1 -or
        $Updated.Body.redirect_uris[0] -ne
        "https://step06.example/exact-callback") {
        throw "exact redirect URI replacement failed"
    }

    Write-Host "PASS set exact redirect URI"

    $Disabled = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications/$AppID/disable" `
        $null `
        $AdminCSRF

    if ($Disabled.Status -ne 200 -or
        $Disabled.Body.enabled -ne $false) {
        throw "disable application failed"
    }

    $Enabled = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications/$AppID/enable" `
        $null `
        $AdminCSRF

    if ($Enabled.Status -ne 200 -or
        $Enabled.Body.enabled -ne $true) {
        throw "enable application failed"
    }

    Write-Host "PASS disable -> enable"

    $Rotated = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications/$AppID/rotate-secret" `
        $null `
        $AdminCSRF

    if ($Rotated.Status -ne 200 -or
        [string]::IsNullOrWhiteSpace(
            $Rotated.Body.client_secret
        ) -or
        $Rotated.Body.client_secret -eq $InitialSecret) {
        throw "client secret rotation failed"
    }

    Write-Host "PASS rotate client secret"

    $AssignmentBody = @{
        role = "user"
    }

    $Assigned = Invoke-AdminJSON `
        "PUT" `
        "/api/identity/applications/$AppID/assignments/$($CurrentUser.id)" `
        $AssignmentBody `
        $AdminCSRF

    if ($Assigned.Status -ne 200 -or
        $Assigned.Body.role -ne "user") {
        throw "assignment upsert failed"
    }

    $Assignments = Invoke-AdminJSON `
        "GET" `
        "/api/identity/applications/$AppID/assignments"

    if ($Assignments.Status -ne 200 -or
        $Assignments.Body.assignments.Count -lt 1) {
        throw "assignment listing failed"
    }

    $DeletedAssignment = Invoke-AdminJSON `
        "DELETE" `
        "/api/identity/applications/$AppID/assignments/$($CurrentUser.id)" `
        $null `
        $AdminCSRF

    if ($DeletedAssignment.Status -ne 204) {
        throw "assignment deletion failed"
    }

    Write-Host "PASS basic Identity assignments"

    $Audit = Invoke-AdminJSON `
        "GET" `
        "/api/identity/audit?limit=200"

    if ($Audit.Status -ne 200) {
        throw "Identity audit listing failed"
    }

    $EventTypes = @(
        $Audit.Body.audit |
        ForEach-Object { $_.event_type }
    )

    foreach ($RequiredEvent in @(
        "identity.application.created",
        "identity.application.updated",
        "identity.application.disabled",
        "identity.application.enabled",
        "identity.application.secret_rotated",
        "identity.assignment.upserted",
        "identity.assignment.deleted"
    )) {
        if ($EventTypes -notcontains $RequiredEvent) {
            throw "audit missing event: $RequiredEvent"
        }
    }

    Write-Host "PASS inspect Identity audit"

    $Archived = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications/$AppID/archive" `
        $null `
        $AdminCSRF

    if ($Archived.Status -ne 200 -or
        $Archived.Body.enabled -ne $false -or
        $null -eq $Archived.Body.archived_at) {
        throw "archive application failed"
    }

    $EnableArchived = Invoke-AdminJSON `
        "POST" `
        "/api/identity/applications/$AppID/enable" `
        $null `
        $AdminCSRF

    if ($EnableArchived.Status -ne 409) {
        throw "archived application was re-enabled"
    }

    Write-Host "PASS archive is terminal"

    Write-Host ""
    Write-Host "STEP 06 E2E PASS"
}
finally {
    Remove-Item $CookieJar -ErrorAction SilentlyContinue
    Remove-Item $Headers -ErrorAction SilentlyContinue
}
