# Secrets for local/staging/prod

For Ansible deployments with `maxbridge_manage_secrets: true`, this folder is usually not edited on the server.

Use this folder primarily for:
1. local Docker Compose runs;
2. controller-side secret sync when `maxbridge_manage_secrets: false`.

## Public-safe templates

Template files without real values are provided in:
`deploy/compose/secrets/examples/*.example`

Copy templates to real secret filenames when needed:
- `db_dsn`
- `postgres_password`
- `invite_hash_pepper`
- `telegram_bot_token`
- `telegram_webhook_secret`
- `max_bot_token`
- `max_webhook_secret`
- `backup_encryption_key`

Rules:
1. Never commit real secret values.
2. Prefer file permissions `0400`.
3. Keep environment-specific secret management details in private ops docs.
