# Operations Runbook

## 1. Локальный запуск
1. Подготовить secret files (см. `deploy/compose/secrets/README.md`).
2. Поднять стек: `docker compose -f deploy/compose/docker-compose.yml up -d`.
3. Применить миграции: `docker compose -f deploy/compose/docker-compose.yml exec bridge /app/bridge migrate up`.
4. Проверить `/health/ready`, `/metrics`.
5. Открыть TUI основной командой: `./scripts/maxbridge`.
6. Поведение wrapper:
   - если `bridge` уже running, используется `docker compose ... exec bridge /app/tui`;
   - если `bridge` не running, используется fallback `docker compose ... run --rm bridge /app/tui`.

## 2. Production deploy (GitOps-lite)
1. CI: lint/test/build immutable images, push registry tag.
2. Ansible deploy:
   - sync compose/env;
   - sync/manage secrets (при `maxbridge_manage_secrets=true`);
   - pull image tag;
   - migrate up;
   - compose up -d;
   - health checks.
3. Secrets flow:
   - внешние `maxbridge_telegram_bot_token` и `maxbridge_max_bot_token` задаются через Vault vars;
   - остальные секреты (`postgres_password`, webhook secrets, invite pepper, backup key) Ansible может сгенерировать один раз и далее переиспользовать.
4. На target host Ansible устанавливает `/usr/local/bin/maxbridge` (операторский TUI wrapper).
5. Rollback:
   - задеплоить предыдущий image tag;
   - `docker compose up -d`;
   - схема БД должна оставаться backward-compatible.

## 3. Retention policy
1. Постоянно хранятся: `telegram_groups`, `max_users`, `routes`, `invites`.
2. Регулярно очищаются:
   - `delivery_jobs`: completed/dead_letter по TTL;
   - `delivery_attempts`: по TTL;
   - `dedupe_records`: по `expires_at`.
3. Payload minimization:
   - payload completed jobs очищается после `RETENTION_PAYLOAD_HOURS`.

## 4. Queue и восстановление
1. Stale `processing` jobs возвращаются в `retry` после lease timeout.
2. Worker поднимает задачу повторно по правилам retry/backoff.
3. Для ручного recovery использовать TUI command: `queue retry <job_id>`.

## 5. Health checks
1. `GET /health/live`.
2. `GET /health/ready`.
3. `GET /health/checks`.
4. В TUI раздел `Health Checks` показывает db/telegram/max/queue.

## 6. Операторские команды TUI
1. `group add|probe|probeall|remove`
2. `invite create|revoke`
3. `route add|pause|resume|delete`
4. `queue retry|clear-completed`
5. `user block|unblock|remove|test`

## 7. Backup schedule
1. Backup запускается таймером `maxbridge-backup.timer`.
2. По умолчанию расписание: `03:10 UTC/local` (переопределяется `maxbridge_backup_schedule`).
3. Backup job читает `db_dsn` и `backup_encryption_key` из `maxbridge_secrets_dir`.
