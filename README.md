# MaxBridge

Production-oriented мостовой сервис Telegram -> MAX с надёжной очередью доставки и администрированием через TUI.

## What it does

1. Принимает Telegram/MAX webhooks с проверкой secret headers.
2. Использует invite-based onboarding для MAX пользователей.
3. Гарантирует durable delivery (at-least-once, retry, DLQ, lease recovery).
4. Поддерживает operator workflows через fullscreen TUI (без web-admin).

## Repository model

1. Репозиторий публичный.
2. Секреты и приватные операционные детали не хранятся в Git.
3. Подробные private runbooks должны храниться вне публичного репозитория.

См.:
- `SECURITY.md`
- `docs/public-repo-policy.md`
- `AGENTS.md`

## Local quick start

1. Подготовьте локальные secret files в `deploy/compose/secrets/` по шаблонам из `deploy/compose/secrets/examples/`.
2. Поднимите стек:

```bash
docker compose -f deploy/compose/docker-compose.yml up -d
```

3. Примените миграции:

```bash
docker compose -f deploy/compose/docker-compose.yml run --rm bridge /app/bridge migrate up
```

4. Откройте TUI:

```bash
./scripts/maxbridge
```

## Deploy model (public-safe)

1. Build/publish image: `.github/workflows/cd-image.yml`.
2. Deploy/rollback: `.github/workflows/cd-deploy.yml`, `.github/workflows/cd-rollback.yml`.
3. Runtime: Docker Compose + Ansible.

Точные environment-specific операционные детали и recovery-нюансы должны быть вынесены в private ops docs.

## Releases

1. Бинарники `bridge` / `worker` / `tui` публикуются через `.github/workflows/release.yml`.
2. Релизные бинарники не хранятся в git-tracked файлах.

## Documentation

1. `docs/project-context.md`
2. `docs/operations.md`
3. `docs/security.md`
4. `docs/public-repo-policy.md`
5. `docs/migration.md`
6. `docs/backup-restore.md`
7. `docs/adr/0001-architecture.md`
