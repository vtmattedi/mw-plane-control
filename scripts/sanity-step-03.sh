#!/usr/bin/env sh
set -eu

PORT="${1:-3050}"
ENV_FILE="${2:-.env.step3.local}"
BASE="http://127.0.0.1:${PORT}"
HOST_NAME="identity.mattediworks.com"

echo "== Step 03 OIDC metadata / registry sanity =="

DISCOVERY="$(
  curl -fsS     -H "Host: ${HOST_NAME}"     "${BASE}/.well-known/openid-configuration"
)"

printf '%s' "$DISCOVERY" |
  grep -Fq '"issuer":"https://identity.mattediworks.com"'
printf '%s' "$DISCOVERY" |
  grep -Fq '"authorization_endpoint":"https://identity.mattediworks.com/oauth/authorize"'
printf '%s' "$DISCOVERY" |
  grep -Fq '"token_endpoint":"https://identity.mattediworks.com/oauth/token"'

echo "PASS OIDC discovery"

JWKS="$(
  curl -fsS     -H "Host: ${HOST_NAME}"     "${BASE}/.well-known/jwks.json"
)"

printf '%s' "$JWKS" | grep -Fq '"alg":"RS256"'
INITIAL_KID="$(
  printf '%s' "$JWKS" |
    sed -n 's/.*"kid":"\([^"]*\)".*/\1/p' |
    head -n 1
)"
test -n "$INITIAL_KID"

echo "PASS JWKS active RSA signing key"

STAMP="$(date +%s)"
CLIENT_ID="mw_admin_step03_${STAMP}"

CREATED="$(
  docker compose --env-file "$ENV_FILE" exec -T mw-identity     /app/create-application     --name "MW Admin Step 03"     --client-id "$CLIENT_ID"     --type confidential     --internal     --redirect-uri "https://admin.mattediworks.com/api/auth/callback"     --scope openid     --scope profile     --scope email     --audience mw-admin     --json
)"

printf '%s' "$CREATED" | grep -Fq ""client_id":"${CLIENT_ID}""
printf '%s' "$CREATED" | grep -Fq '"client_secret":"mw_client_'

echo "PASS application registry + one-time client secret"

docker compose --env-file "$ENV_FILE" exec -T mw-identity   /app/rotate-signing-key

echo "PASS signing-key rotation command"

JWKS_AFTER="$(
  curl -fsS     -H "Host: ${HOST_NAME}"     "${BASE}/.well-known/jwks.json"
)"

printf '%s' "$JWKS_AFTER" | grep -Fq ""kid":"${INITIAL_KID}""

COUNT="$(
  printf '%s' "$JWKS_AFTER" |
    grep -o '"kid":"' |
    wc -l |
    tr -d ' '
)"
test "$COUNT" -ge 2

echo "PASS JWKS retains retired verification key"

echo
echo "== Step 02 regression =="
"$(dirname "$0")/sanity-step-02.sh" "$PORT"

echo
echo "STEP 03 E2E PASS"
