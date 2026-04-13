# Project Context: MaxBridge

Updated: 2026-04-03
Path: `docs/project-context.md`

## 1. Цель проекта

`MaxBridge` — production-oriented мост `Telegram -> MAX` с fullscreen TUI-управлением (без web-admin).

Ключевые требования:
1. Надёжная доставка (at-least-once, retry/backoff, DLQ, recovery).
2. Invite-based onboarding для MAX пользователей.
3. Не отвечать и не сохранять мусор от неавторизованных MAX пользователей.
4. Контроль роста БД через retention/cleanup.
5. Повторяемое развёртывание: Docker Compose + Ansible.

## 2. Архитектура (high level)

### 2.1 Процессы
1. `cmd/bridge`: HTTP ingress (webhooks), health/metrics, housekeeping.
2. `cmd/worker`: обработка delivery queue, retry/DLQ/recovery.
3. `cmd/tui`: fullscreen Bubble Tea UI для операторских операций.

### 2.2 Внутренние модули
1. `internal/httpapi` — webhook handlers и health API.
2. `internal/storage` — PostgreSQL store + queue/migration logic.
3. `internal/delivery` — worker engine.
4. `internal/invites` — invite code generation/hash/parsing.
5. `internal/telegram` — Telegram API client.
6. `internal/max` — MAX API client.
7. `internal/tui` — UI model + admin service.
8. `internal/ops` — retention/partitions scheduler.
9. `internal/app` — config/logger/metrics.
10. `internal/domain` — доменные типы/правила.

## 3. Критичные потоки и инварианты

### 3.1 Telegram ingress -> queue
1. `POST /webhooks/telegram`.
2. Проверка Telegram secret header.
3. Валидация payload и ограничение размера.
4. Быстрый durable enqueue в `delivery_jobs`.
5. Быстрый ACK webhook.

### 3.2 MAX link onboarding
1. `POST /webhooks/max`.
2. Проверка MAX secret header.
3. Обработка только `/link <invite_code>`.
4. Invalid/unknown link: no reply + no DB writes.
5. Valid link: consume invite + link user + test send + status update.

### 3.3 Worker delivery lifecycle
1. Claim jobs (`pending|retry`) с lease.
2. Отправка в MAX с bounded concurrency.
3. Успех -> `completed`.
4. Temporary error -> `retry` c backoff.
5. Permanent/exhausted -> `dead_letter`.
6. Stale `processing` после lease timeout -> requeue.

## 4. HTTP и CLI

### 4.1 HTTP
1. `POST /webhooks/telegram`
2. `POST /webhooks/max`
3. `GET /health/live`
4. `GET /health/ready`
5. `GET /health/checks`
6. `GET /metrics`

### 4.2 CLI
1. `bridge serve`
2. `bridge migrate up|down`
3. `bridge health`
4. `worker run`
5. `tui`

## 5. Данные (PostgreSQL)

Основные таблицы:
1. `telegram_groups`
2. `max_users`
3. `invites`
4. `routes`
5. `dedupe_records`
6. `delivery_jobs`
7. `delivery_attempts`
8. `app_events`

Ключевые ограничения:
1. `routes` unique `(telegram_chat_id, max_user_id)`.
2. `invites.code_hash` unique.
3. `dedupe_records.dedupe_key` unique.
4. `delivery_jobs.status` enum-like check.

## 6. Безопасность и минимизация данных

1. Секреты читаются из файлов (`*_FILE`).
2. Проверяются webhook secret headers Telegram/MAX.
3. DB не публикуется наружу.
4. Invalid/unauthorized MAX traffic не сохраняется.
5. Invite code хранится в hash-виде.
6. Payload очищается по retention-политике.

## 7. Deploy/CI model (public-safe)

1. Runtime: Docker Compose + Nginx + PostgreSQL.
2. Provision/deploy: Ansible playbooks.
3. CI/CD source of truth: GitHub Actions workflows в `.github/workflows/`.
4. Автоматический delivery pipeline: `ci` -> `cd-image` -> `cd-deploy` (staging checks -> production promotion).
5. При провале production deploy/checks выполняется автоматический rollback на предыдущий `sha-*` image tag.
6. Required checks должны быть blocking для merge в `main`.
7. Детальные environment-specific операции, runner setup и recovery-runbooks держать в private ops docs.

## 8. Backup/restore

1. `scripts/backup-db.sh`
2. `scripts/restore-db.sh <backup.enc>`
3. `scripts/verify-backup.sh <backup.enc>`
4. Политика: `pg_dump -Fc` + шифрование backup + регулярная проверка восстановления.

## 9. Связанные документы

1. `docs/operations.md`
2. `docs/security.md`
3. `docs/public-repo-policy.md`
4. `docs/migration.md`
5. `docs/backup-restore.md`
6. `docs/adr/0001-architecture.md`
