# MaxBridge

Production-oriented мостовой сервис Telegram -> MAX.

## Возможности

1. Приём Telegram webhook'ов с проверкой секретных заголовков и надёжной постановкой в очередь.
2. Онбординг MAX через `/link <invite_code>` с тихим игнорированием невалидного и неавторизованного трафика.
3. Invite-коды хранятся только в виде хеша; исходный код показывается один раз при создании.
4. Надёжная очередь доставки с семантикой at-least-once, дедупликацией, повторными попытками, DLQ и восстановлением lease.
5. Полноэкранное администрирование через TUI (Bubble Tea) без web-admin панели.
6. Задачи retention и cleanup для контроля роста БД.
7. Запуск через Docker Compose и provision/deploy через Ansible (GitOps-lite).

## Структура репозитория

```text
cmd/
  bridge/
  worker/
  tui/
internal/
  app/ domain/ telegram/ max/ routing/ delivery/ invites/ storage/ ops/ httpapi/ tui/
migrations/
deploy/
  compose/
  ansible/
docs/
scripts/
tests/
```

## Локальный запуск

1. Создайте файлы секретов в `deploy/compose/secrets/`.
2. Запустите стек:

```bash
docker compose -f deploy/compose/docker-compose.yml up -d
```

3. Выполните миграции:

```bash
docker compose -f deploy/compose/docker-compose.yml run --rm bridge /app/bridge migrate up
```

4. Откройте TUI:

```bash
./scripts/maxbridge
```

## Продакшен-деплой

Source of truth для CI/CD: **SourceCraft Free**.
GitHub/GitLab для этого репозитория используются как read-only mirrors (опционально).

1. Настройте SourceCraft secrets/variables:
   - protected/shared: `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`, `MAXBRIDGE_IMAGE_REPO`
   - environment `staging`: `MAXBRIDGE_TELEGRAM_BOT_TOKEN`, `MAXBRIDGE_MAX_BOT_TOKEN`, `MAXBRIDGE_REGISTRY_TOKEN`, `MAXBRIDGE_DOMAIN`, `MAXBRIDGE_HTTPS_PORT`, `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY_PATH`, `DEPLOY_SSH_KNOWN_HOSTS_PATH`
   - environment `production`: те же ключи, что и `staging`
2. Запустите SourceCraft workflows из `.sourcecraft/ci.yaml`:
   - `image-publish` для публикации immutable image tags (`sha-*`, `main`, `v*`)
   - `deploy-staging`/`rollback-staging` вручную
   - `deploy-production`/`rollback-production` вручную с confirm-переменной
3. Проверьте:

```bash
curl -k https://<domain>:8443/health/ready
```

4. Откройте TUI на сервере:

```bash
maxbridge
```

## Откат

1. Запустите SourceCraft workflow `rollback-staging` или `rollback-production` с `ROLLBACK_IMAGE_TAG`.
2. Проверьте `/health/ready` и `/health/checks`.

## Восстановление на новом сервере

1. Выполните bootstrap хоста через Ansible.
2. Скопируйте зашифрованный бэкап и ключ.
3. Восстановите базу данных через `scripts/restore-db.sh`.
4. Разверните compose-стек с тем же тегом образа.
5. Выполните health-check'и и переключите DNS.

## Документация

0. `docs/project-context.md`
1. `docs/operations.md`
2. `docs/security.md`
3. `docs/migration.md`
4. `docs/backup-restore.md`
5. `docs/refs.md`
6. `docs/NOTES.md`
7. `docs/adr/0001-architecture.md`
8. `docs/sourcecraft-migration.md`
