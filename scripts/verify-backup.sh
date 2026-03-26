#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: verify-backup.sh <encrypted_dump.enc>" >&2
  exit 1
fi

ENC_FILE="$1"
TMP_DUMP="/tmp/maxbridge-verify-$(date -u +%s).dump"
ROOT_DIR="/opt/maxbridge"
BACKUP_ENCRYPTION_KEY_FILE="${BACKUP_ENCRYPTION_KEY_FILE:-${ROOT_DIR}/secrets/backup_encryption_key}"

openssl enc -d -aes-256-cbc -pbkdf2 -in "$ENC_FILE" -out "$TMP_DUMP" -pass file:"$BACKUP_ENCRYPTION_KEY_FILE"
pg_restore --list "$TMP_DUMP" >/dev/null
rm -f "$TMP_DUMP"

echo "backup verification passed: $ENC_FILE"

