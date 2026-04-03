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

1. Настройте GitHub Environments:
   - `shared`: secrets `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`
   - `staging`: secrets `MAXBRIDGE_TELEGRAM_BOT_TOKEN`, `MAXBRIDGE_MAX_BOT_TOKEN`, `MAXBRIDGE_REGISTRY_TOKEN`, variables `MAXBRIDGE_DOMAIN`, `MAXBRIDGE_HTTPS_PORT`, `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY_PATH`, `DEPLOY_SSH_KNOWN_HOSTS_PATH`
   - `production`: те же secrets/variables, что и `staging`
2. Задайте repo variable `MAXBRIDGE_IMAGE_REPO` (например `docker.io/<user>/maxbridge`).
3. Соберите и опубликуйте immutable image tag через workflow `cd-image`.
4. Выполните deploy через workflow `cd-deploy`:
   - `environment`: `staging`/`production`
   - `image_tag`: `sha-*` или release tag
   - `run_bootstrap`: `true` только для первичной подготовки host
5. Проверьте:

```bash
curl -k https://<domain>:8443/health/ready
```

6. Откройте TUI на сервере:

```bash
maxbridge
```

## Релизные бинарники

1. Бинарники не хранятся в git-tracked файлах репозитория.
2. Публикация `bridge` / `worker` / `tui` выполняется через workflow `.github/workflows/release.yml`.
3. Workflow запускается по git tag `v*` и прикрепляет артефакты к GitHub Release.

## Откат

1. Запустите workflow `cd-rollback` с `environment` и предыдущим `image_tag`.
2. Проверьте `/health/ready` и метрики очереди.

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
8. `docs/security-preflight-2026-04-03.md`
