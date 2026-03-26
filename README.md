# MaxBridge

Production-oriented bridge service Telegram -> MAX.

## Features

1. Telegram webhook ingestion with secret header validation and durable enqueue.
2. MAX webhook onboarding via `/link <invite_code>` with silent ignore for invalid/unauthorized traffic.
3. Invite codes are stored as hash only; raw code shown once on creation.
4. Durable delivery queue with at-least-once semantics, dedupe, retries, DLQ, and lease recovery.
5. Fullscreen TUI administration (Bubble Tea) without web admin panel.
6. Data retention and cleanup jobs to control DB growth.
7. Docker Compose runtime + Ansible provisioning/deploy (GitOps-lite).

## Repository layout

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

## Local run

1. Create secret files in `deploy/compose/secrets/`.
2. Start stack:

```bash
docker compose -f deploy/compose/docker-compose.yml up -d
```

3. Run migrations:

```bash
docker compose -f deploy/compose/docker-compose.yml run --rm bridge /app/bridge migrate up
```

4. Open TUI:

```bash
docker compose -f deploy/compose/docker-compose.yml run --rm bridge /app/tui
```

## Production deploy

1. Build and push immutable image tag in CI.
2. Put required tokens into Ansible Vault (`deploy/ansible/group_vars/all/vault.yml`):

```yaml
maxbridge_telegram_bot_token: "<telegram_token>"
maxbridge_max_bot_token: "<max_token>"
```

For private Docker Hub repos also add:

```yaml
maxbridge_registry_token: "<docker_hub_access_token>"
```

3. Run Ansible:

```bash
ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/bootstrap.yml
ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/deploy.yml \
  --ask-vault-pass \
  -e "maxbridge_version=<tag>" \
  -e "maxbridge_image=docker.io/<user>/maxbridge:<tag>" \
  -e "maxbridge_domain=<domain>"
```

4. If image repo is private, set in `deploy/ansible/group_vars/all/base.yml`:

```yaml
maxbridge_registry_private: true
maxbridge_registry_url: "https://index.docker.io/v1/"
maxbridge_registry_username: "<docker_hub_user>"
```

5. Verify:

```bash
curl -k https://<domain>:8443/health/ready
```

## Rollback

1. Deploy previous image tag:

```bash
ansible-playbook -i deploy/ansible/inventory/hosts.yml deploy/ansible/deploy.yml -e "maxbridge_version=<prev_tag>"
```

2. Validate `/health/ready` and queue metrics.

## Restore on new server

1. Bootstrap host with Ansible.
2. Copy encrypted backup and key.
3. Restore database via `scripts/restore-db.sh`.
4. Deploy compose stack with same image tag.
5. Run health checks and switch DNS.

## Docs

0. `docs/project-context.md`
1. `docs/operations.md`
2. `docs/security.md`
3. `docs/migration.md`
4. `docs/backup-restore.md`
5. `docs/refs.md`
6. `docs/NOTES.md`
7. `docs/adr/0001-architecture.md`
