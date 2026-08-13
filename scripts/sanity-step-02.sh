#!/usr/bin/env sh
set -eu

PORT="${1:-3050}"
BASE="http://127.0.0.1:${PORT}"
HOST_NAME="identity.mattediworks.com"
STAMP="$(date +%s)"
EMAIL="step02-${STAMP}@mattediworks.test"
PASSWORD="Step02-Integration-Password-${STAMP}"
COOKIE_JAR="$(mktemp)"

cleanup() {
  rm -f "$COOKIE_JAR"
}
trap cleanup EXIT

echo "== Step 02 Identity local-auth sanity =="

printf '%s\n' "$PASSWORD" |
  docker compose exec -T mw-identity     /app/create-superadmin     --email "$EMAIL"     --display-name "Step 02 Admin"     --password-stdin

echo "PASS create-superadmin CLI"

LOGIN_PAGE="$(
  curl -fsS     -c "$COOKIE_JAR"     -H "Host: ${HOST_NAME}"     "${BASE}/login"
)"

LOGIN_CSRF="$(
  printf '%s' "$LOGIN_PAGE" |
    sed -n 's/.*name="csrf_token" value="\([^"]*\)".*/\1/p' |
    head -n 1
)"

test -n "$LOGIN_CSRF"
echo "PASS login page + pre-auth CSRF"

LOGIN_STATUS="$(
  curl -sS     -o /dev/null     -w '%{http_code}'     -b "$COOKIE_JAR"     -c "$COOKIE_JAR"     -H "Host: ${HOST_NAME}"     -H 'Content-Type: application/x-www-form-urlencoded'     --data-urlencode "email=${EMAIL}"     --data-urlencode "password=${PASSWORD}"     --data-urlencode "csrf_token=${LOGIN_CSRF}"     "${BASE}/login"
)"

test "$LOGIN_STATUS" = "303"
echo "PASS password login"

ACCOUNT_PAGE="$(
  curl -fsS     -b "$COOKIE_JAR"     -c "$COOKIE_JAR"     -H "Host: ${HOST_NAME}"     "${BASE}/account"
)"

printf '%s' "$ACCOUNT_PAGE" | grep -Fq "$EMAIL"

LOGOUT_CSRF="$(
  printf '%s' "$ACCOUNT_PAGE" |
    sed -n 's/.*name="csrf_token" value="\([^"]*\)".*/\1/p' |
    head -n 1
)"

test -n "$LOGOUT_CSRF"
echo "PASS protected session"

BAD_STATUS="$(
  curl -sS     -o /dev/null     -w '%{http_code}'     -b "$COOKIE_JAR"     -c "$COOKIE_JAR"     -H "Host: ${HOST_NAME}"     -H 'Content-Type: application/x-www-form-urlencoded'     --data-urlencode 'csrf_token=wrong-token'     "${BASE}/logout"
)"
test "$BAD_STATUS" = "403"
echo "PASS logout CSRF rejection"

LOGOUT_STATUS="$(
  curl -sS     -o /dev/null     -w '%{http_code}'     -b "$COOKIE_JAR"     -c "$COOKIE_JAR"     -H "Host: ${HOST_NAME}"     -H 'Content-Type: application/x-www-form-urlencoded'     --data-urlencode "csrf_token=${LOGOUT_CSRF}"     "${BASE}/logout"
)"
test "$LOGOUT_STATUS" = "303"
echo "PASS logout"

AFTER="$(
  curl -sS     -o /dev/null     -w '%{http_code}'     -b "$COOKIE_JAR"     -H "Host: ${HOST_NAME}"     "${BASE}/account"
)"
test "$AFTER" = "303"
echo "PASS session invalidated"

echo
echo "STEP 02 E2E PASS"
