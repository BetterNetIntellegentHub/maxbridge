# Project Context: MaxBridge

Updated: 2026-04-01
Path: `docs/project-context.md`

## 1. Цель проекта

`MaxBridge` — production-oriented мост `Telegram -> MAX` с управлением через fullscreen TUI (без web-admin).

Ключевые требования:
1. Надежная доставка (at-least-once, retry/backoff, DLQ, recovery).
2. Invite-based onboarding для MAX пользователей.
3. Не отвечать и не сохранять мусор от неавторизованных MAX пользователей.
4. Контроль роста БД через retention/cleanup.
5. Простое single-server развёртывание: Docker Compose + Ansible (GitOps-lite).

## 2. Текущая архитектура

### 2.1 Процессы
1. `cmd/bridge`:
   - HTTP ingress (Telegram/MAX webhooks)
   - health/metrics endpoints
   - housekeeping (retention + partition maintenance)
2. `cmd/worker`:
   - обработка delivery queue
   - rate limit + circuit breaker
   - retry/DLQ + lease recovery
3. `cmd/tui`:
   - fullscreen Bubble Tea UI
   - меню + командный режим для операторских операций

### 2.2 Внутренние модули
1. `internal/httpapi` — webhook handlers и health API.
2. `internal/storage` — PostgreSQL store + queue/migration logic.
3. `internal/delivery` — worker engine.
4. `internal/invites` — invite code generation/hash/parsing.
5. `internal/telegram` — Telegram API client + group probe + deeplink helper.
6. `internal/max` — MAX API client (send/ping).
7. `internal/tui` — UI model + admin service.
8. `internal/ops` — retention/partitions scheduler.
9. `internal/app` — config/logger/metrics.
10. `internal/domain` — доменные типы/правила.

## 3. Основные потоки

### 3.1 Telegram ingress -> queue
1. `POST /webhooks/telegram`.
2. Проверка `X-Telegram-Bot-Api-Secret-Token`.
3. Валидация тела и ограничение размера.
4. Авто-регистрация Telegram групп в `telegram_groups` при update событиях (`message`, `edited_message`, `channel_post`, `my_chat_member`, `chat_member`) для `group/supergroup`.
5. Быстрое durable enqueue в `delivery_jobs` (через route + dedupe).
6. Быстрый ответ webhook.
7. Формат пересылки в MAX: заголовок `"<Telegram chat title> - <Telegram sender>"`, затем текст/подпись; вложения Telegram (photo/document/video/audio/voice/animation) пересылаются как реальные attachments через `POST /uploads` -> `POST /messages`.

### 3.2 MAX link onboarding
1. `POST /webhooks/max`.
2. Проверка `X-Max-Bot-Api-Secret`.
3. Обработка только `/link <invite_code>` (команда извлекается из `message.text` или `message.body.text` MAX update).
4. Invalid/unknown link:
   - no reply
   - no DB writes
   - только counter `invalid_link_ignored_total`.
5. Valid link:
   - consume invite (hash-based)
   - upsert linked user (имя берётся из metadata инвайта `max_full_name`)
   - optional auto-route binding by scope
   - immediate test send в MAX
   - update user delivery status

### 3.3 Worker delivery lifecycle
1. Claim jobs (`pending|retry`) с lease (`FOR UPDATE SKIP LOCKED`).
2. Отправка в MAX с bounded concurrency.
3. Успех -> `completed` + attempt record.
4. Temporary error -> backoff + `retry`.
5. Permanent/exhausted -> `dead_letter`.
6. Stale `processing` after lease timeout -> requeue.

## 4. API и endpoints

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
5. `tui` (fullscreen)

## 5. TUI sections и интерактивные действия

### 5.1 Секции
1. Dashboard
2. Telegram Groups
3. MAX Users
4. Invites
5. Routes
6. Delivery Queue
7. Health Checks
8. Logs
9. Settings
10. Exit

### 5.2 Навигация и операции
1. Модель управления: `Enter` открывает следующий уровень меню (`раздел -> элемент -> список действий`).
2. На каждом уровне отображается только текущий список (без одновременного показа других меню).
3. Возврат реализован отдельным явным пунктом `Back`, который всегда расположен последним в текущем списке.
4. В разделах с данными (`Telegram Groups`, `MAX Users`, `Invites`, `Routes`, `Delivery Queue`) доступны:
   - действия для выбранной записи;
   - действия уровня раздела (например, add/create/cleanup), которые показываются сразу в списке раздела без промежуточного пункта `Section Actions`.
5. Операции, требующие параметров, выполняются через встроенные формы ввода в TUI (без `:` режима).
   - В формах действия выполняются через явный пункт `Сохранить`; пункт `Назад` расположен ниже.
   - Для `Добавить маршрут` ручной ввод ID не используется: выбор идёт из интерактивных списков существующих Telegram групп и MAX пользователей.
6. Потенциально destructive-операции требуют явного подтверждения (`y/enter` для выполнения, `n/esc` для отмены).
7. Базовые клавиши:
   - `Enter` — открыть/выбрать;
   - `Esc` — шаг назад;
   - `r` — обновить текущий список;
   - `q` — выход.
8. Для `Добавить маршрут` в UI применяются безопасные defaults: `filter_mode=all`, `ignore_bot_messages=true` (без ручного ввода этих параметров).
9. `Создать инвайт` в разделе `Invites` открывает форму с обязательным полем `Имя пользователя MAX`; TTL/scope остаются безопасными defaults (`24h`, `entity:general`).
10. Использованные и отозванные инвайты удаляются из БД и не отображаются в списках TUI.
11. В `MAX Users` доступен row action `Переименовать` для ручного обновления имени пользователя.
12. Row action `Удалить пользователя` в `MAX Users` выполняет soft-delete (`is_active=false`) и отключает все связанные маршруты (`routes.enabled=false`) без физического удаления исторических данных.

## 6. Схема данных (PostgreSQL)

Основные таблицы:
1. `telegram_groups`
2. `max_users` (`full_name` для user-friendly отображения в TUI)
3. `invites` (hash + raw code + `max_full_name` в metadata для операторского отображения в TUI; used/revoked инвайты удаляются)
4. `routes`
5. `dedupe_records` (unique `dedupe_key`)
6. `delivery_jobs`
7. `delivery_attempts` (partitioned by month)
8. `app_events`

Важные ограничения:
1. `routes` unique `(telegram_chat_id, max_user_id)`.
2. `invites.code_hash` unique.
3. `dedupe_records.dedupe_key` unique.
4. `delivery_jobs.status` enum-like check.

## 7. Retention / cleanup

Реализовано в housekeeping:
1. Wipe payload у `completed` jobs старше `RETENTION_PAYLOAD_HOURS`.
2. Удаление `delivery_attempts` старше `RETENTION_JOBS_DAYS`.
3. Удаление `completed|dead_letter` jobs старше `RETENTION_JOBS_DAYS`.
4. Очистка `dedupe_records` по `expires_at`/TTL.
5. `ensure_delivery_attempt_partitions()` для обслуживания партиций.

## 8. Метрики

Экспортируются через Prometheus:
1. `telegram_updates_total`
2. `telegram_events_enqueued_total`
3. `max_send_success_total`
4. `max_send_failure_total`
5. `retry_total`
6. `dead_letter_total`
7. `queue_depth`
8. `oldest_pending_job_age`
9. `max_api_latency`
10. `db_errors_total`
11. `invalid_webhook_total`
12. `invalid_link_ignored_total`
13. `successful_links_total`

## 9. Безопасность

1. Секреты через files (`*_FILE`), не через CLI args.
2. Логгер редактирует `token/secret/password/invite` поля.
3. Nginx публикует только HTTPS, webhook endpoints POST-only.
4. Проверяются secret headers Telegram/MAX.
5. `client_max_body_size` + `limit_req` настроены в Nginx.
6. БД не публикуется наружу.

## 10. Deploy / GitOps-lite

### 10.1 Локально
1. Подготовить `deploy/compose/secrets/*`.
2. `docker compose -f deploy/compose/docker-compose.yml up -d`
3. `docker compose -f deploy/compose/docker-compose.yml run --rm bridge /app/bridge migrate up`
4. `./scripts/maxbridge` (если `bridge` запущен — используется `docker compose ... exec`, иначе fallback `docker compose ... run --rm`)

### 10.2 Прод
1. `ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/bootstrap.yml`
2. `ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/deploy.yml -e "maxbridge_version=<tag>" -e "maxbridge_domain=<domain>"`
3. При `maxbridge_manage_secrets=true` Ansible сам управляет секретами на target host; внешние токены (`maxbridge_telegram_bot_token`, `maxbridge_max_bot_token`, `maxbridge_registry_token`) для CD приходят из GitHub Environment secrets в runtime `--extra-vars`.
4. Compose `.env` формируется Ansible (`BRIDGE_IMAGE`, `NGINX_HTTPS_PORT`), что позволяет переопределять host HTTPS port (например `8443`, если `443` занят).
5. Для private registry задаются `maxbridge_registry_private=true`, `maxbridge_registry_username`, `maxbridge_registry_url`, а `maxbridge_registry_token` передается из environment secret; перед pull Ansible делает `docker login` и валидирует наличие creds в private-режиме.
6. При деплое нового образа задавать `maxbridge_image` явно (например `docker.io/argusvlad/maxbridge:<tag>`), иначе может использоваться default placeholder registry.
7. Для публикации образов `cd-image` берет registry credentials из `shared` environment secrets.
8. Ansible устанавливает `/usr/local/bin/maxbridge` (wrapper для TUI): если сервис `bridge` запущен — `exec`, иначе fallback `run --rm`.
9. После пересоздания `bridge` возможен кратковременный `502` на внешнем `health/ready` из-за stale upstream в Nginx; рабочий обход — рестарт `compose-nginx-1`.
10. После успешного `docker push`/deploy выполняется безопасная очистка неиспользуемых артефактов:
   - локально: Docker build cache, неиспользуемые образы, Go cache/modcache, `%TEMP%` (`AppData\\Local\\Temp`) и рабочие временные файлы;
   - на сервере: только неиспользуемые Docker images/cache/stopped containers;
   - активные контейнеры и используемые тома не удаляются.
   - post-check: `docker system df` и проверка свободного места на `C:`/host; на сервере дополнительно проверяется `docker ps`.
   - если свободное место на `C:` не увеличивается после cleanup, проверяются крупные `*.vhdx` (Docker/WSL), для них применяется отдельная процедура compact по явному подтверждению оператора.

### 10.3 Rollback
1. Повторный deploy с предыдущим immutable tag.

### 10.4 CI / GitHub Actions
1. Основной workflow: `.github/workflows/ci.yml` (`actionlint`, `secret-scan`, `vuln-scan`, `staticcheck`, `gosec`, `test-build`).
2. Для `secret-scan` используется `gitleaks/gitleaks-action@v2` (blocking).
3. Для воспроизводимости `vuln-scan` не используется `govulncheck@latest`; применяется pinned-инструмент:
   - `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`
4. В `vuln-scan` используется отдельный toolchain `actions/setup-go@v6` с `go-version: "1.25.8"` (ветка `1.25.x`) для стабильного security-сканирования.
5. В `staticcheck` и `gosec` используются pinned-версии инструментов:
   - `honnef.co/go/tools/cmd/staticcheck@v0.6.1`
   - `github.com/securego/gosec/v2/cmd/gosec@v2.22.4`
6. В `test-build` используется `actions/setup-go@v6` с `go-version-file: go.mod`, чтобы версия тестов/сборки была детерминирована репозиторием.
7. Все CI checks остаются blocking для PR/merge; `continue-on-error` не используется.
8. `CodeQL` выключен для текущего режима GitHub Free + private repository (infra-ограничение платформы). Возврат `CodeQL` возможен отдельным change set при смене плана/модели репозитория.
9. CD workflows:
   - `.github/workflows/cd-image.yml`: build/push immutable tags (`sha-*`, `main`, `v*`), generate SBOM (Syft), blocking Trivy scan.
   - `.github/workflows/cd-deploy.yml`: manual deploy (`workflow_dispatch`) через Ansible (`deploy.yml`) по выбранному `image_tag`.
   - `.github/workflows/cd-rollback.yml`: manual rollback по предыдущему immutable `image_tag` через тот же deploy path.
10. Secrets/vars contract:
   - `shared` env secrets: `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`.
   - `staging`/`production` env secrets: `MAXBRIDGE_TELEGRAM_BOT_TOKEN`, `MAXBRIDGE_MAX_BOT_TOKEN`, `MAXBRIDGE_REGISTRY_TOKEN`.
   - `staging`/`production` env vars: `MAXBRIDGE_DOMAIN`, `MAXBRIDGE_HTTPS_PORT`, `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY_PATH`, `DEPLOY_SSH_KNOWN_HOSTS_PATH`.
   - repo var: `MAXBRIDGE_IMAGE_REPO`.
11. Runner routing:
   - `cd-image` runs on GitHub-hosted runner.
   - `cd-deploy` / `cd-rollback` run on self-hosted labels `self-hosted`, `Linux`, `X64`, `wsl-deploy` (WSL runner).
12. Production guardrails for GitHub Free:
   - actor allowlist: only `BetterNetIntellegentHub`;
   - explicit confirmation input required:
     - deploy: `production_confirm=DEPLOY_PRODUCTION`
     - rollback: `production_confirm=ROLLBACK_PRODUCTION`.

## 11. Backup/restore

Скрипты:
1. `scripts/backup-db.sh`
2. `scripts/restore-db.sh <backup.enc>`
3. `scripts/verify-backup.sh <backup.enc>`

Политика:
1. `pg_dump -Fc` + шифрование backup.
2. Ежедневный запуск через `maxbridge-backup.timer`.
3. Retention backup через cleanup.

## 12. Документы-источники в репо

1. `agents.md` (правила действий агента)
2. `README.md`
3. `docs/adr/0001-architecture.md`
4. `docs/refs.md`
5. `docs/NOTES.md`
6. `docs/operations.md`
7. `docs/security.md`
8. `docs/migration.md`
9. `docs/backup-restore.md`
10. `docs/assumptions.md`

## 13. Известные ограничения/незавершённость

1. Полноценные integration/e2e тесты пока placeholders (`tests/integration`, `tests/e2e`).
2. Полная runtime валидация с `docker compose` и реальными webhooks в этой среде не выполнена (нет docker runtime); compile-валидация Go команд и модулей выполнялась.
3. MAX webhook payload schema может требовать дополнительной адаптации под конкретные event variants.
4. TUI реализован как fullscreen interactive menu; полная runtime-проверка UX в production-окружении не выполнялась в этой среде.

## 14. Правило работы с контекстом

1. При изменениях архитектуры/схемы/операционных процедур сначала обновлять этот файл.
2. При новых задачах использовать этот файл как baseline и сверять с конкретными исходниками.
3. Если возникает противоречие между этим файлом и кодом, приоритет у кода + обновление файла в том же change set.



