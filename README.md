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

1. Соберите и опубликуйте неизменяемый тег образа в CI.
2. Поместите необходимые токены в Ansible Vault (`deploy/ansible/group_vars/all/vault.yml`):

```yaml
maxbridge_telegram_bot_token: "<telegram_token>"
maxbridge_max_bot_token: "<max_token>"
```

Для приватных Docker Hub репозиториев также добавьте:

```yaml
maxbridge_registry_token: "<docker_hub_access_token>"
```

3. Запустите Ansible:

```bash
ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/bootstrap.yml
ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/deploy.yml \
  --ask-vault-pass \
  -e "maxbridge_version=<tag>" \
  -e "maxbridge_image=docker.io/<user>/maxbridge:<tag>" \
  -e "maxbridge_domain=<domain>"
```

4. Если репозиторий образов приватный, задайте в `deploy/ansible/group_vars/all/base.yml`:

```yaml
maxbridge_registry_private: true
maxbridge_registry_url: "https://index.docker.io/v1/"
maxbridge_registry_username: "<docker_hub_user>"
```

5. Проверьте:

```bash
curl -k https://<domain>:8443/health/ready
```

6. Откройте TUI на сервере:

```bash
maxbridge
```

## Откат

1. Задеплойте предыдущий тег образа:

```bash
ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/deploy.yml -e "maxbridge_version=<prev_tag>"
```

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
