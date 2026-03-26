#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: configure-telegram-webhook.sh <bot_token> <public_base_url> <secret>" >&2
  exit 1
fi

BOT_TOKEN="$1"
BASE_URL="$2"
SECRET="$3"

curl -sS -X POST "https://api.telegram.org/bot${BOT_TOKEN}/setWebhook" \
  -H 'Content-Type: application/json' \
  -d "{\"url\":\"${BASE_URL}/webhooks/telegram\",\"secret_token\":\"${SECRET}\",\"allowed_updates\":[\"message\"]}"

echo

