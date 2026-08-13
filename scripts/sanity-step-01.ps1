param(
    [int]$Port = 3050
)

$ErrorActionPreference = "Stop"
$Base = "http://127.0.0.1:$Port"

function Invoke-CurlText {
    param(
        [string]$HostName,
        [string]$Path
    )

    $text = & curl.exe `
        --fail `
        --silent `
        --show-error `
        -H "Host: $HostName" `
        "$Base$Path"

    if ($LASTEXITCODE -ne 0) {
        throw "curl failed for $HostName$Path"
    }

    return ($text -join "`n")
}

Write-Host "== Step 01 runtime sanity =="

$admin = Invoke-CurlText `
    -HostName "admin.mattediworks.com" `
    -Path "/"

if ($admin -notmatch "MW Plane Control Admin Test") {
    throw "Admin placeholder did not come from the gateway static host."
}
Write-Host "PASS admin static host"

$bffRaw = Invoke-CurlText `
    -HostName "admin.mattediworks.com" `
    -Path "/api/health"
$bff = $bffRaw | ConvertFrom-Json

if ($bff.status -ne "ok" -or $bff.service -ne "mw-bff") {
    throw "BFF health response was unexpected: $bffRaw"
}
Write-Host "PASS admin /api/health -> MW BFF"

$identityRaw = Invoke-CurlText `
    -HostName "identity.mattediworks.com" `
    -Path "/health"
$identity = $identityRaw | ConvertFrom-Json

if (
    $identity.status -ne "ok" -or
    $identity.service -ne "mw-identity"
) {
    throw "Identity health response was unexpected: $identityRaw"
}
Write-Host "PASS identity /health -> MW Identity"

$portal = Invoke-CurlText `
    -HostName "identity.mattediworks.com" `
    -Path "/"

if ($portal -notmatch "MW Identity") {
    throw "Identity portal HTML was unexpected."
}
Write-Host "PASS identity portal"

$status = & curl.exe `
    --silent `
    --output NUL `
    --write-out "%{http_code}" `
    -H "Host: unexpected.example" `
    "$Base/"

if ($LASTEXITCODE -ne 0) {
    throw "curl failed while checking unknown host"
}

if ($status -ne "404") {
    throw "Unknown host returned HTTP $status; expected 404."
}
Write-Host "PASS unknown Host -> 404"

Write-Host ""
docker compose ps

Write-Host ""
Write-Host "STEP 01 E2E PASS"
