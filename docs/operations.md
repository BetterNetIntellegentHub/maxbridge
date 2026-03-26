# Operations Runbook

## 1. Локальный запуск
1. Подготовить secrets files (см. `deploy/compose/secrets/README.md`).
2. Поднять БД и сервисы: `docker compose -f deploy/compose/docker-compose.yml up -d`.
3. Применить миграции: `docker compose -f deploy/compose/docker-compose.yml exec bridge /app/bridge migrate up`.
4. Проверить: `/health/ready`, `/metrics`.
5. Запустить TUI: `docker compose -f deploy/compose/docker-compose.yml exec bridge /app/tui`.

## 2. Production deploy (GitOps-lite)
1. CI: lint/test/build immutable images, push registry tag.
2. Ansible deploy:
   - раскладка compose/env
   - управление secrets (при `maxbridge_manage_secrets=true`)
   - pull image tag
   - migrate up
   - compose up -d
   - health checks
3. Secrets flow без ручной работы на сервере:
   - хранить `maxbridge_telegram_bot_token` и `maxbridge_max_bot_token` в Vault vars
   - остальные локальные секреты (`postgres_password`, webhook secrets, invite pepper, backup key) Ansible может сгенерировать автоматически
4. Rollback:
   - сменить image tag на предыдущий
   - `docker compose up -d`
   - БД миграции только backward-compatible.

## 3. Retention policy
1. Долгоживущие таблицы: `telegram_groups`, `max_users`, `routes`, `invites`.
2. Ограниченное хранение:
   - `delivery_jobs`: completed/dead_letter очищаются по TTL
   - `delivery_attempts`: очищаются по TTL
   - `dedupe_records`: по `expires_at`
3. Payload minimization:
   - payload в completed jobs обнуляется после `RETENTION_PAYLOAD_HOURS`.

## 4. Очередь и восстановление
1. Зависшие `processing` jobs возвращаются в `retry` через lease timeout.
2. Worker перезапуск не требует ручного вмешательства.
3. Для ручной операции использовать TUI command: `queue retry <job_id>`.

## 5. Health checks
1. `GET /health/live`.
2. `GET /health/ready`.
3. `GET /health/checks`.
4. TUI раздел `Health Checks` должен показывать db/telegram/max/queue.

## 6. Эксплуатационные команды TUI
1. `group add|probe|probeall|remove`
2. `invite create|revoke`
3. `route add|pause|resume|delete`
4. `queue retry|clear-completed`
5. `user block|unblock|remove|test`

## 7. Backup schedule
1. Роль backup ставит `maxbridge-backup.timer`.
2. По умолчанию расписание: `03:10 UTC/local` (настраивается `maxbridge_backup_schedule`).
3. Скрипт читает `db_dsn` и `backup_encryption_key` из `maxbridge_secrets_dir`.