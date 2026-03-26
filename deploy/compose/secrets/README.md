# Secrets for local/staging/prod

For Ansible deployments with `maxbridge_manage_secrets: true`, you usually do not edit this folder on the server.
Secrets are generated/written by Ansible to `maxbridge_secrets_dir` and synced automatically.

Use this folder mainly when `maxbridge_manage_secrets: false` or for local Docker Compose runs.

Put secrets as plain text files (one value per file):
- `db_dsn`
- `postgres_password`
- `invite_hash_pepper`
- `telegram_bot_token`
- `telegram_webhook_secret`
- `max_bot_token`
- `max_webhook_secret`
- `backup_encryption_key`

`db_dsn` example:
`postgres://maxbridge:<postgres_password>@postgres:5432/maxbridge?sslmode=disable`

Rules:
- Never commit real secret values to git.
- Rotate webhook secrets and API tokens if they were ever exposed.
- Prefer file permissions `0400`.