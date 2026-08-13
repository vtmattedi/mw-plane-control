param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$Base = "http://127.0.0.1:$Port"
$HostName = "identity.mattediworks.com"
$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$Email = "step02-$Stamp@mattediworks.test"
$Password = "Step02-Integration-Password-$Stamp"
$CookieJar = Join-Path $env:TEMP "mw-plane-step02-$Stamp.cookies.txt"

try {
    Write-Host "== Step 02 Identity local-auth sanity =="

    $Password | docker compose exec -T mw-identity `
        /app/create-superadmin `
        --email $Email `
        --display-name "Step 02 Admin" `
        --password-stdin

    if ($LASTEXITCODE -ne 0) {
        throw "create-superadmin failed"
    }

    Write-Host "PASS create-superadmin CLI"

    $LoginPage = & curl.exe `
        --fail `
        --silent `
        --show-error `
        -c $CookieJar `
        -H "Host: $HostName" `
        "$Base/login"

    if ($LASTEXITCODE -ne 0) {
        throw "GET /login failed"
    }

    $Match = [regex]::Match(
        ($LoginPage -join "`n"),
        'name="csrf_token" value="([^"]+)"'
    )

    if (-not $Match.Success) {
        throw "login CSRF token was not found in HTML"
    }

    $LoginCSRF = $Match.Groups[1].Value
    Write-Host "PASS login page + pre-auth CSRF"

    $LoginStatus = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "email=$Email" `
        --data-urlencode "password=$Password" `
        --data-urlencode "csrf_token=$LoginCSRF" `
        "$Base/login"

    if ($LASTEXITCODE -ne 0) {
        throw "POST /login failed"
    }
    if ($LoginStatus -ne "303") {
        throw "POST /login returned HTTP $LoginStatus; expected 303"
    }

    Write-Host "PASS password login"

    $AccountPage = & curl.exe `
        --fail `
        --silent `
        --show-error `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        "$Base/account"

    if ($LASTEXITCODE -ne 0) {
        throw "GET /account failed"
    }

    $AccountText = $AccountPage -join "`n"
    if ($AccountText -notmatch [regex]::Escape($Email)) {
        throw "protected account page did not contain current identity"
    }

    $LogoutMatch = [regex]::Match(
        $AccountText,
        'name="csrf_token" value="([^"]+)"'
    )
    if (-not $LogoutMatch.Success) {
        throw "logout CSRF token was not found"
    }

    $LogoutCSRF = $LogoutMatch.Groups[1].Value
    Write-Host "PASS protected session"

    $BadCSRFStatus = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "csrf_token=wrong-token" `
        "$Base/logout"

    if ($LASTEXITCODE -ne 0) {
        throw "bad-CSRF logout request failed unexpectedly"
    }
    if ($BadCSRFStatus -ne "403") {
        throw "bad CSRF returned HTTP $BadCSRFStatus; expected 403"
    }

    Write-Host "PASS logout CSRF rejection"

    $LogoutStatus = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "csrf_token=$LogoutCSRF" `
        "$Base/logout"

    if ($LASTEXITCODE -ne 0) {
        throw "POST /logout failed"
    }
    if ($LogoutStatus -ne "303") {
        throw "POST /logout returned HTTP $LogoutStatus; expected 303"
    }

    Write-Host "PASS logout"

    $AfterLogout = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -H "Host: $HostName" `
        "$Base/account"

    if ($LASTEXITCODE -ne 0) {
        throw "post-logout account check failed"
    }
    if ($AfterLogout -ne "303") {
        throw "account after logout returned HTTP $AfterLogout; expected 303"
    }

    Write-Host "PASS session invalidated"

    Write-Host ""
    Write-Host "STEP 02 E2E PASS"
}
finally {
    Remove-Item $CookieJar -ErrorAction SilentlyContinue
}
