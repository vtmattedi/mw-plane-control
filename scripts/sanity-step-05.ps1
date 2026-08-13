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
$Email = "step05-$Stamp@mattediworks.test"
$Password = "Step05-Integration-Password-$Stamp"

$CookieJar = Join-Path $env:TEMP "mw-step05-$Stamp.cookies"
$Headers = Join-Path $env:TEMP "mw-step05-$Stamp.headers"

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
        throw "request failed for $HostName$Path"
    }

    $Line = Get-Content $Headers |
        Where-Object { $_ -match '^Location:\s*(.+)\s*$' } |
        Select-Object -First 1

    if ($null -eq $Line) {
        throw "Location header missing for $HostName$Path"
    }

    return @{
        Status = $Status
        Location = [regex]::Match(
            $Line,
            '^Location:\s*(.+?)\s*$'
        ).Groups[1].Value
    }
}

function URL-PathAndQuery([string]$URL) {
    $Parsed = [System.Uri]$URL
    return $Parsed.PathAndQuery
}

try {
    Write-Host "== Step 05 BFF OIDC login =="

    $Password | docker compose --env-file $EnvFile exec -T mw-identity `
        /app/create-superadmin `
        --email $Email `
        --display-name "Step 05 Admin" `
        --password-stdin

    if ($LASTEXITCODE -ne 0) {
        throw "create-superadmin failed"
    }

    Write-Host "PASS bootstrap Identity admin"

    $BFFLogin = Request-Redirect `
        $AdminHost `
        "/api/auth/login?return_to=%2F"

    if ($BFFLogin.Status -ne "302") {
        throw "BFF login HTTP $($BFFLogin.Status); expected 302"
    }
    if (-not $BFFLogin.Location.StartsWith(
        "https://identity.mattediworks.com/oauth/authorize?"
    )) {
        throw "BFF did not redirect to MW Identity"
    }

    Write-Host "PASS BFF created state/nonce/PKCE transaction"

    $AuthorizePath = URL-PathAndQuery $BFFLogin.Location

    $IdentityAuth = Request-Redirect `
        $IdentityHost `
        $AuthorizePath

    if ($IdentityAuth.Status -ne "303" -or
        $IdentityAuth.Location -notmatch '^/login\?return_to=') {
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

    if ($LASTEXITCODE -ne 0) {
        throw "Identity login page failed"
    }

    $CSRF = [regex]::Match(
        $LoginPage,
        'name="csrf_token" value="([^"]+)"'
    ).Groups[1].Value

    $ReturnToEncoded = [regex]::Match(
        $LoginPage,
        'name="return_to" value="([^"]+)"'
    ).Groups[1].Value

    if ([string]::IsNullOrWhiteSpace($CSRF) -or
        [string]::IsNullOrWhiteSpace($ReturnToEncoded)) {
        throw "Identity login form missing CSRF/return_to"
    }

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
        throw "Identity login HTTP $LoginStatus; expected 303"
    }

    Write-Host "PASS Identity password login"

    $IdentityComplete = Request-Redirect `
        $IdentityHost `
        $AuthorizePath

    if ($IdentityComplete.Status -ne "302") {
        throw (
            "Identity authorization HTTP " +
            "$($IdentityComplete.Status); expected 302"
        )
    }

    if (-not $IdentityComplete.Location.StartsWith(
        "https://admin.mattediworks.com/api/auth/callback?"
    )) {
        throw "Identity callback target is not Admin BFF"
    }

    Write-Host "PASS Identity issued authorization code to BFF callback"

    $CallbackPath = URL-PathAndQuery $IdentityComplete.Location

    $BFFCallback = Request-Redirect `
        $AdminHost `
        $CallbackPath

    if ($BFFCallback.Status -ne "303") {
        throw "BFF callback HTTP $($BFFCallback.Status); expected 303"
    }
    if ($BFFCallback.Location -ne
        "https://admin.mattediworks.com/") {
        throw "BFF callback returned unexpected final redirect"
    }

    Write-Host "PASS BFF code exchange + ID-token validation + session"

    $MeRaw = & curl.exe `
        --fail `
        --silent `
        --show-error `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $AdminHost" `
        "$Base/api/auth/me"

    if ($LASTEXITCODE -ne 0) {
        throw "/api/auth/me failed"
    }

    $Me = ($MeRaw -join "`n") | ConvertFrom-Json

    if ($Me.identity.issuer -ne
        "https://identity.mattediworks.com") {
        throw "BFF session issuer mismatch"
    }
    if ($Me.identity.email -ne $Email) {
        throw "BFF session email mismatch"
    }
    if ([string]::IsNullOrWhiteSpace(
        $Me.identity.sub
    )) {
        throw "BFF session subject missing"
    }

    Write-Host "PASS BFF /auth/me"

    $CSRFResponse = (
        (& curl.exe `
            --fail `
            --silent `
            --show-error `
            -X POST `
            -b $CookieJar `
            -c $CookieJar `
            -H "Host: $AdminHost" `
            "$Base/api/auth/csrf") -join "`n"
    ) | ConvertFrom-Json

    $AdminCSRF = $CSRFResponse.csrf_token
    if ([string]::IsNullOrWhiteSpace($AdminCSRF)) {
        throw "BFF CSRF token missing"
    }

    Write-Host "PASS BFF CSRF rotation"

    $BadLogout = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -X POST `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $AdminHost" `
        -H "X-MW-CSRF: definitely-wrong" `
        "$Base/api/auth/logout"

    if ($BadLogout -ne "403") {
        throw "bad BFF CSRF HTTP $BadLogout; expected 403"
    }

    Write-Host "PASS BFF logout CSRF rejection"

    $LogoutStatus = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -X POST `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $AdminHost" `
        -H "X-MW-CSRF: $AdminCSRF" `
        "$Base/api/auth/logout"

    if ($LogoutStatus -ne "204") {
        throw "BFF logout HTTP $LogoutStatus; expected 204"
    }

    $AfterLogout = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -H "Host: $AdminHost" `
        "$Base/api/auth/me"

    if ($AfterLogout -ne "401") {
        throw "BFF session remained valid after logout"
    }

    Write-Host "PASS BFF logout + session invalidation"

    Write-Host ""
    Write-Host "STEP 05 E2E PASS"
}
finally {
    Remove-Item $CookieJar -ErrorAction SilentlyContinue
    Remove-Item $Headers -ErrorAction SilentlyContinue
}
