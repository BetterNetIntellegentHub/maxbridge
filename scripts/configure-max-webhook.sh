#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 4 ]]; then
  echo "usage: configure-max-webhook.sh <max_token> <public_base_url> <secret> <callback_url_optional>" >&2
  exit 1
fi

MAX_TOKEN="$1"
BASE_URL="$2"
SECRET="$3"
CALLBACK_URL="${4:-${BASE_URL}/webhooks/max}"
AUTH_TOKEN="${MAX_TOKEN#Bearer }"
AUTH_TOKEN="${AUTH_TOKEN#bearer }"
AUTH_TOKEN="$(printf '%s' "${AUTH_TOKEN}" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"

curl -sS -X POST "https://botapi.max.ru/subscriptions" \
  -H "Authorization: ${AUTH_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"url\":\"${CALLBACK_URL}\",\"secret\":\"${SECRET}\"}"

echo

