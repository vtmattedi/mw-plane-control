#!/usr/bin/env sh
set -eu

PORT="${1:-3050}"
ENV_FILE=".env.step3.local"
export GATEWAY_HOST_PORT="$PORT"

if [ ! -f "$ENV_FILE" ]; then
  KEY="$(
    head -c 32 /dev/urandom |
      base64 |
      tr '+/' '-_' |
      tr -d '=\n'
  )"

  cat > "$ENV_FILE" <<EOF
APP_ENV=development
GATEWAY_HOST_PORT=$PORT
IDENTITY_PUBLIC_ISSUER=https://identity.mattediworks.com
IDENTITY_SIGNING_KEYS=1:$KEY
IDENTITY_SIGNING_ACTIVE_KEY_VERSION=1
EOF
  echo "created local ignored env: $ENV_FILE"
fi

docker compose --env-file "$ENV_FILE" up -d --build

i=0
while [ "$i" -lt 40 ]; do
  if curl -fsS     -H 'Host: identity.mattediworks.com'     "http://127.0.0.1:${PORT}/health" >/dev/null 2>&1; then
    exec "$(dirname "$0")/sanity-step-03.sh" "$PORT" "$ENV_FILE"
  fi

  i=$((i + 1))
  sleep 1
done

docker compose --env-file "$ENV_FILE" ps
docker compose --env-file "$ENV_FILE" logs --tail=150
echo "Identity did not become healthy in time" >&2
exit 1
