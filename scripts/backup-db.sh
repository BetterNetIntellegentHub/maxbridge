#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="/opt/maxbridge"
BACKUP_DIR="${ROOT_DIR}/backup/archive"
DATE_TAG="$(date -u +%Y%m%dT%H%M%SZ)"
TMP_DUMP="${BACKUP_DIR}/maxbridge-${DATE_TAG}.dump"
ENC_DUMP="${TMP_DUMP}.enc"

DB_DSN_FILE="${DB_DSN_FILE:-${ROOT_DIR}/secrets/db_dsn}"
BACKUP_ENCRYPTION_KEY_FILE="${BACKUP_ENCRYPTION_KEY_FILE:-${ROOT_DIR}/secrets/backup_encryption_key}"

if [[ ! -f "$DB_DSN_FILE" ]]; then
  echo "missing DB_DSN_FILE: $DB_DSN_FILE" >&2
  exit 1
fi
if [[ ! -f "$BACKUP_ENCRYPTION_KEY_FILE" ]]; then
  echo "missing BACKUP_ENCRYPTION_KEY_FILE: $BACKUP_ENCRYPTION_KEY_FILE" >&2
  exit 1
fi

DB_DSN="$(tr -d '\r\n' < "$DB_DSN_FILE")"
mkdir -p "$BACKUP_DIR"

pg_dump "$DB_DSN" -Fc -f "$TMP_DUMP"
openssl enc -aes-256-cbc -pbkdf2 -salt -in "$TMP_DUMP" -out "$ENC_DUMP" -pass file:"$BACKUP_ENCRYPTION_KEY_FILE"
rm -f "$TMP_DUMP"

find "$BACKUP_DIR" -type f -name '*.enc' -mtime +30 -delete

echo "backup created: $ENC_DUMP"

