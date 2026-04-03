#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: restore-db.sh <encrypted_dump.enc>" >&2
  exit 1
fi

ENC_FILE="$1"
TMP_DUMP="/tmp/maxbridge-restore-$(date -u +%s).dump"

ROOT_DIR="/opt/maxbridge"
DB_DSN_FILE="${DB_DSN_FILE:-${ROOT_DIR}/secrets/db_dsn}"
BACKUP_ENCRYPTION_KEY_FILE="${BACKUP_ENCRYPTION_KEY_FILE:-${ROOT_DIR}/secrets/backup_encryption_key}"

DB_DSN="$(tr -d '\r\n' < "$DB_DSN_FILE")"
openssl enc -d -aes-256-cbc -pbkdf2 -in "$ENC_FILE" -out "$TMP_DUMP" -pass file:"$BACKUP_ENCRYPTION_KEY_FILE"

pg_restore --clean --if-exists --no-owner --no-privileges -d "$DB_DSN" "$TMP_DUMP"
rm -f "$TMP_DUMP"

echo "restore completed from: $ENC_FILE"

