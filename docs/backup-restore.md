# Backup & Restore

## Backup policy
1. Ежедневный full backup PostgreSQL (`pg_dump -Fc`).
2. Шифрование backup через `openssl` (`aes-256-cbc`, pbkdf2).
3. Retention по умолчанию: удаление `.enc` старше 30 дней (можно адаптировать).
4. Регулярная проверка восстановления в staging.

## Scripts
1. `scripts/backup-db.sh`
2. `scripts/restore-db.sh`
3. `scripts/verify-backup.sh`

## Runtime wiring
1. Ansible роль `backup` ставит `maxbridge-backup.service` и `maxbridge-backup.timer`.
2. Service использует:
   - `DB_DSN_FILE={{ maxbridge_secrets_dir }}/db_dsn`
   - `BACKUP_ENCRYPTION_KEY_FILE={{ maxbridge_secrets_dir }}/backup_encryption_key`
3. Backup архивы сохраняются в `{{ maxbridge_app_dir }}/backup/archive`.

## Restore procedure
1. Подготовить чистую БД.
2. Расшифровать backup.
3. Выполнить `pg_restore --clean --if-exists --no-owner --no-privileges`.
4. Проверить `bridge health` и выборочно данные.

## Scenarios
1. Потеря сервера: bootstrap + restore + deploy + DNS switch.
2. Неудачная миграция: app rollback + selective DB recovery.
3. Переезд на новый сервер: dry-run restore перед cutover.