#!/usr/bin/env sh
set -eu

PORT="${1:-3050}"
export GATEWAY_HOST_PORT="$PORT"

echo "== Rebuild and start Step 02 =="
docker compose up -d --build

echo
echo "== Wait for Identity health =="

i=0
while [ "$i" -lt 30 ]; do
  if curl -fsS     -H 'Host: identity.mattediworks.com'     "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
    exec "$(dirname "$0")/sanity-step-02.sh" "$PORT"
  fi

  i=$((i + 1))
  sleep 1
done

docker compose ps
docker compose logs --tail=120
echo "Identity did not become healthy in time" >&2
exit 1
