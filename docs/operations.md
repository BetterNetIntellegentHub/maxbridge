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
2. CD workflows:
   - `.github/workflows/cd-image.yml`: build/push immutable tags + SBOM + blocking Trivy scan.
   - `.github/workflows/cd-deploy.yml`: manual deploy by `environment` + `image_tag`.
   - `.github/workflows/cd-rollback.yml`: manual rollback by `environment` + previous `image_tag`.
3. Secrets and vars source in GitHub:
   - `shared` environment secrets: `REGISTRY_USERNAME`, `REGISTRY_PASSWORD`.
   - `staging`/`production` environment secrets: `MAXBRIDGE_TELEGRAM_BOT_TOKEN`, `MAXBRIDGE_MAX_BOT_TOKEN`, `MAXBRIDGE_REGISTRY_TOKEN`.
   - `staging`/`production` environment vars: `MAXBRIDGE_DOMAIN`, `MAXBRIDGE_HTTPS_PORT`, `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY_PATH`, `DEPLOY_SSH_KNOWN_HOSTS_PATH`.
   - repo var: `MAXBRIDGE_IMAGE_REPO`.
4. Runner model:
   - `cd-image` runs on GitHub-hosted (`ubuntu-latest`).
   - `cd-deploy` / `cd-rollback` run on self-hosted runner labels: `self-hosted`, `Linux`, `X64`, `wsl-deploy`.
   - deploy availability depends on local WSL runner being online.
5. Ansible deploy path in workflows:
   - sync compose/env;
   - runtime `--extra-vars` with bot/registry secrets;
   - sync/manage host secrets (при `maxbridge_manage_secrets=true`);
   - pull image tag;
   - migrate up;
   - compose up -d;
   - health checks.
6. CD post-check model:
   - external readiness check: `https://${MAXBRIDGE_DOMAIN}:${MAXBRIDGE_HTTPS_PORT}/health/ready`;
   - metrics sanity check is executed over SSH on target host via `https://127.0.0.1/metrics` (localhost allowlist в Nginx).
7. На target host Ansible устанавливает `/usr/local/bin/maxbridge` (операторский TUI wrapper).
8. Manual fallback path:
   - допускается запуск playbook с Vault (`group_vars/all/vault.yml`) вне GitHub Actions.
9. Production guardrails (GitHub Free fallback):
   - only actor `BetterNetIntellegentHub` can run production deploy/rollback;
   - explicit confirmation input is mandatory:
     - deploy: `production_confirm=DEPLOY_PRODUCTION`
     - rollback: `production_confirm=ROLLBACK_PRODUCTION`
10. Rollback:
   - задеплоить предыдущий image tag;
   - `docker compose up -d`;
   - схема БД должна оставаться backward-compatible.

## 2.1 WSL self-hosted runner operations
1. Service name: `actions.runner.BetterNetIntellegentHub-maxbridge.wsl-maxbridge.service`.
2. Status check:
   - `systemctl status actions.runner.BetterNetIntellegentHub-maxbridge.wsl-maxbridge.service`
3. Logs:
   - `journalctl -u actions.runner.BetterNetIntellegentHub-maxbridge.wsl-maxbridge.service -n 200 --no-pager`
4. Restart:
   - `sudo systemctl restart actions.runner.BetterNetIntellegentHub-maxbridge.wsl-maxbridge.service`
5. Temporary deploy freeze:
   - `sudo systemctl stop actions.runner.BetterNetIntellegentHub-maxbridge.wsl-maxbridge.service`
6. Re-enable after freeze:
   - `sudo systemctl start actions.runner.BetterNetIntellegentHub-maxbridge.wsl-maxbridge.service`

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
