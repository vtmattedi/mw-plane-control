#!/usr/bin/env sh
set -eu

PORT="${1:-3050}"
BASE="http://127.0.0.1:${PORT}"

echo "== Step 01 runtime sanity =="

admin="$(curl -fsS -H 'Host: admin.mattediworks.com' "${BASE}/")"
printf '%s' "$admin" | grep -q 'MW Plane Control Admin Test'
echo "PASS admin static host"

bff="$(curl -fsS -H 'Host: admin.mattediworks.com' "${BASE}/api/health")"
printf '%s' "$bff" | grep -q '"status":"ok"'
printf '%s' "$bff" | grep -q '"service":"mw-bff"'
echo "PASS admin /api/health -> MW BFF"

identity="$(curl -fsS -H 'Host: identity.mattediworks.com' "${BASE}/health")"
printf '%s' "$identity" | grep -q '"status":"ok"'
printf '%s' "$identity" | grep -q '"service":"mw-identity"'
echo "PASS identity /health -> MW Identity"

portal="$(curl -fsS -H 'Host: identity.mattediworks.com' "${BASE}/")"
printf '%s' "$portal" | grep -q 'MW Identity'
echo "PASS identity portal"

status="$(
  curl -sS -o /dev/null -w '%{http_code}'     -H 'Host: unexpected.example'     "${BASE}/"
)"
test "$status" = "404"
echo "PASS unknown Host -> 404"

echo
docker compose ps
echo
echo "STEP 01 E2E PASS"
