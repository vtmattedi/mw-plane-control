param(
    [int]$Port = 3050,
    [Parameter(Mandatory = $true)]
    [string]$EnvFile
)

$ErrorActionPreference = "Stop"

$Base = "http://127.0.0.1:$Port"
$HostName = "identity.mattediworks.com"
$RedirectURI = "https://client.example/callback"
$Stamp = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
$Email = "step04-$Stamp@mattediworks.test"
$Password = "Step04-Integration-Password-$Stamp"
$ClientID = "step04_client_$Stamp"
$CookieJar = Join-Path $env:TEMP "mw-step04-$Stamp.cookies"
$Headers = Join-Path $env:TEMP "mw-step04-$Stamp.headers"

function To-B64Url([byte[]]$Bytes) {
    return [Convert]::ToBase64String($Bytes).
        TrimEnd('=').
        Replace('+', '-').
        Replace('/', '_')
}

function From-B64Url([string]$Value) {
    $Value = $Value.Replace('-', '+').Replace('_', '/')
    switch ($Value.Length % 4) {
        2 { $Value += "==" }
        3 { $Value += "=" }
        0 { }
        default { throw "invalid base64url" }
    }
    return [Convert]::FromBase64String($Value)
}

function New-PKCE {
    $Bytes = New-Object byte[] 64
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($Bytes)
    $Verifier = To-B64Url $Bytes

    $SHA = [System.Security.Cryptography.SHA256]::Create()
    try {
        $Digest = $SHA.ComputeHash(
            [System.Text.Encoding]::ASCII.GetBytes($Verifier)
        )
    }
    finally {
        $SHA.Dispose()
    }

    return @{
        Verifier = $Verifier
        Challenge = (To-B64Url $Digest)
    }
}

function Q([string]$Value) {
    return [System.Uri]::EscapeDataString($Value)
}

function Request-Redirect([string]$Path) {
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
        throw "request failed: $Path"
    }

    $Line = Get-Content $Headers |
        Where-Object { $_ -match '^Location:\s*(.+)\s*$' } |
        Select-Object -First 1

    if ($null -eq $Line) {
        throw "Location missing for $Path"
    }

    return @{
        Status = $Status
        Location = [regex]::Match(
            $Line,
            '^Location:\s*(.+?)\s*$'
        ).Groups[1].Value
    }
}

function Query-Value(
    [string]$URL,
    [string]$Name
) {
    $Match = [regex]::Match(
        $URL,
        "(?:\?|&)$([regex]::Escape($Name))=([^&]+)"
    )

    if (-not $Match.Success) {
        throw "$Name missing from callback"
    }

    return [System.Uri]::UnescapeDataString(
        $Match.Groups[1].Value.Replace('+', ' ')
    )
}

function Verify-IDToken(
    [string]$Token,
    [string]$ExpectedClient,
    [string]$ExpectedNonce,
    [string]$ExpectedEmail
) {
    $Parts = $Token.Split('.')
    if ($Parts.Count -ne 3) {
        throw "ID token is not compact JWS"
    }

    $Header = (
        [System.Text.Encoding]::UTF8.GetString(
            (From-B64Url $Parts[0])
        )
    ) | ConvertFrom-Json

    $Payload = (
        [System.Text.Encoding]::UTF8.GetString(
            (From-B64Url $Parts[1])
        )
    ) | ConvertFrom-Json

    $JWKS = (
        (& curl.exe `
            --fail `
            --silent `
            --show-error `
            -H "Host: $HostName" `
            "$Base/.well-known/jwks.json") -join "`n"
    ) | ConvertFrom-Json

    $JWK = $JWKS.keys |
        Where-Object { $_.kid -eq $Header.kid } |
        Select-Object -First 1

    if ($null -eq $JWK -or $Header.alg -ne "RS256") {
        throw "ID token signing key/alg invalid"
    }

    $Parameters = [System.Security.Cryptography.RSAParameters]::new()
    $Parameters.Modulus = From-B64Url $JWK.n
    $Parameters.Exponent = From-B64Url $JWK.e

    $RSA = [System.Security.Cryptography.RSA]::Create()
    try {
        $RSA.ImportParameters($Parameters)

        $Valid = $RSA.VerifyData(
            [System.Text.Encoding]::ASCII.GetBytes(
                "$($Parts[0]).$($Parts[1])"
            ),
            (From-B64Url $Parts[2]),
            [System.Security.Cryptography.HashAlgorithmName]::SHA256,
            [System.Security.Cryptography.RSASignaturePadding]::Pkcs1
        )
    }
    finally {
        $RSA.Dispose()
    }

    if (-not $Valid) {
        throw "ID token signature invalid"
    }
    if ($Payload.iss -ne "https://identity.mattediworks.com") {
        throw "ID token issuer invalid"
    }
    if ($Payload.aud -ne $ExpectedClient) {
        throw "ID token audience invalid"
    }
    if ($Payload.nonce -ne $ExpectedNonce) {
        throw "ID token nonce invalid"
    }
    if ($Payload.email -ne $ExpectedEmail) {
        throw "ID token email invalid"
    }

    return $Payload
}

try {
    Write-Host "== Step 04 Authorization Code + PKCE =="

    $Password | docker compose --env-file $EnvFile exec -T mw-identity `
        /app/create-superadmin `
        --email $Email `
        --display-name "Step 04 Admin" `
        --password-stdin

    if ($LASTEXITCODE -ne 0) {
        throw "create-superadmin failed"
    }

    $CreatedRaw = docker compose --env-file $EnvFile exec -T mw-identity `
        /app/create-application `
        --name "Step 04 Client" `
        --client-id $ClientID `
        --type confidential `
        --redirect-uri $RedirectURI `
        --scope openid `
        --scope profile `
        --scope email `
        --json

    if ($LASTEXITCODE -ne 0) {
        throw "create-application failed"
    }

    $Created = ($CreatedRaw -join "`n") | ConvertFrom-Json
    $ClientSecret = $Created.client_secret

    Write-Host "PASS bootstrap user + confidential client"

    $PKCE = New-PKCE
    $State = "state-$Stamp"
    $Nonce = "nonce-$Stamp"

    $AuthorizePath = (
        "/oauth/authorize" +
        "?response_type=code" +
        "&client_id=$(Q $ClientID)" +
        "&redirect_uri=$(Q $RedirectURI)" +
        "&scope=$(Q 'openid profile email')" +
        "&state=$(Q $State)" +
        "&nonce=$(Q $Nonce)" +
        "&code_challenge=$(Q $PKCE.Challenge)" +
        "&code_challenge_method=S256"
    )

    $Unauth = Request-Redirect $AuthorizePath
    if ($Unauth.Status -ne "303" -or
        $Unauth.Location -notmatch '^/login\?return_to=') {
        throw "unauthenticated authorize did not go to Identity login"
    }

    Write-Host "PASS authorize -> login continuation"

    $LoginPage = (& curl.exe `
        --fail `
        --silent `
        --show-error `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        "$Base$($Unauth.Location)") -join "`n"

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
        throw "login form did not preserve authorization request"
    }

    $ReturnTo = [System.Net.WebUtility]::HtmlDecode($ReturnToEncoded)

    $LoginStatus = & curl.exe `
        --silent `
        --show-error `
        --output NUL `
        --dump-header $Headers `
        --write-out "%{http_code}" `
        -b $CookieJar `
        -c $CookieJar `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "email=$Email" `
        --data-urlencode "password=$Password" `
        --data-urlencode "csrf_token=$CSRF" `
        --data-urlencode "return_to=$ReturnTo" `
        "$Base/login"

    if ($LoginStatus -ne "303") {
        throw "login HTTP $LoginStatus; expected 303"
    }

    $Authorized = Request-Redirect $AuthorizePath
    if ($Authorized.Status -ne "302" -or
        -not $Authorized.Location.StartsWith($RedirectURI)) {
        throw "authorization callback redirect invalid"
    }

    $Code = Query-Value $Authorized.Location "code"
    if ((Query-Value $Authorized.Location "state") -ne $State) {
        throw "state mismatch"
    }

    Write-Host "PASS authorization code + state"

    $TokenRaw = & curl.exe `
        --fail `
        --silent `
        --show-error `
        -u "${ClientID}:$ClientSecret" `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "grant_type=authorization_code" `
        --data-urlencode "code=$Code" `
        --data-urlencode "redirect_uri=$RedirectURI" `
        --data-urlencode "code_verifier=$($PKCE.Verifier)" `
        "$Base/oauth/token"

    if ($LASTEXITCODE -ne 0) {
        throw "token exchange failed"
    }

    $Token = ($TokenRaw -join "`n") | ConvertFrom-Json
    if ($Token.token_type -ne "Bearer" -or
        [string]::IsNullOrWhiteSpace($Token.access_token) -or
        [string]::IsNullOrWhiteSpace($Token.id_token)) {
        throw "token response invalid"
    }

    Write-Host "PASS code -> tokens"

    $Payload = Verify-IDToken `
        $Token.id_token `
        $ClientID `
        $Nonce `
        $Email

    Write-Host "PASS ID token signature/claims against JWKS"

    $UserInfo = (
        (& curl.exe `
            --fail `
            --silent `
            --show-error `
            -H "Host: $HostName" `
            -H "Authorization: Bearer $($Token.access_token)" `
            "$Base/oauth/userinfo") -join "`n"
    ) | ConvertFrom-Json

    if ($UserInfo.sub -ne $Payload.sub -or
        $UserInfo.email -ne $Email) {
        throw "userinfo mismatch"
    }

    Write-Host "PASS access token -> userinfo"

    $Replay = (& curl.exe `
        --silent `
        --show-error `
        --write-out "`n%{http_code}" `
        -u "${ClientID}:$ClientSecret" `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "grant_type=authorization_code" `
        --data-urlencode "code=$Code" `
        --data-urlencode "redirect_uri=$RedirectURI" `
        --data-urlencode "code_verifier=$($PKCE.Verifier)" `
        "$Base/oauth/token") -join "`n"

    if ($Replay -notmatch '"error":"invalid_grant"' -or
        $Replay -notmatch '400\s*$') {
        throw "authorization code replay was not rejected"
    }

    Write-Host "PASS one-time authorization code"

    $PKCE2 = New-PKCE
    $AuthorizePath2 = (
        "/oauth/authorize" +
        "?response_type=code" +
        "&client_id=$(Q $ClientID)" +
        "&redirect_uri=$(Q $RedirectURI)" +
        "&scope=$(Q 'openid')" +
        "&state=$(Q "state2-$Stamp")" +
        "&nonce=$(Q "nonce2-$Stamp")" +
        "&code_challenge=$(Q $PKCE2.Challenge)" +
        "&code_challenge_method=S256"
    )

    $Authorized2 = Request-Redirect $AuthorizePath2
    $Code2 = Query-Value $Authorized2.Location "code"

    $BadClient = (& curl.exe `
        --silent `
        --show-error `
        --write-out "`n%{http_code}" `
        -u "${ClientID}:wrong-secret" `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "grant_type=authorization_code" `
        --data-urlencode "code=$Code2" `
        --data-urlencode "redirect_uri=$RedirectURI" `
        --data-urlencode "code_verifier=$($PKCE2.Verifier)" `
        "$Base/oauth/token") -join "`n"

    if ($BadClient -notmatch '"error":"invalid_client"' -or
        $BadClient -notmatch '401\s*$') {
        throw "bad confidential client secret was not rejected"
    }

    $BadPKCE = (& curl.exe `
        --silent `
        --show-error `
        --write-out "`n%{http_code}" `
        -u "${ClientID}:$ClientSecret" `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "grant_type=authorization_code" `
        --data-urlencode "code=$Code2" `
        --data-urlencode "redirect_uri=$RedirectURI" `
        --data-urlencode "code_verifier=$('x' * 64)" `
        "$Base/oauth/token") -join "`n"

    if ($BadPKCE -notmatch '"error":"invalid_grant"') {
        throw "bad PKCE verifier was not rejected"
    }

    & curl.exe `
        --fail `
        --silent `
        --show-error `
        -u "${ClientID}:$ClientSecret" `
        -H "Host: $HostName" `
        -H "Content-Type: application/x-www-form-urlencoded" `
        --data-urlencode "grant_type=authorization_code" `
        --data-urlencode "code=$Code2" `
        --data-urlencode "redirect_uri=$RedirectURI" `
        --data-urlencode "code_verifier=$($PKCE2.Verifier)" `
        "$Base/oauth/token" | Out-Null

    if ($LASTEXITCODE -ne 0) {
        throw "good code was consumed by failed validation"
    }

    Write-Host "PASS client auth + PKCE validation before consume"

    Write-Host ""
    Write-Host "STEP 04 E2E PASS"
}
finally {
    Remove-Item $CookieJar -ErrorAction SilentlyContinue
    Remove-Item $Headers -ErrorAction SilentlyContinue
}
