# Security Notes

## Secrets
1. Секреты не хранятся в Git.
2. Используются file-based secrets mounts (read-only).
3. Секреты не передаются через CLI args.
4. Логи редактируют поля token/secret/password/invite.
5. Для CI/CD используются GitHub Environments secrets (не repo-level):
   - `shared`: registry push credentials;
   - `staging`/`production`: deploy bot/registry tokens.
6. В workflow секреты маскируются и используются только runtime через временный vars-файл.

## Public cutover (2026-04-03)
1. Перед переводом репозитория в public выполнен preflight:
   - `gitleaks git . --redact --no-banner` (история);
   - `gitleaks dir . --redact --no-banner` (текущий HEAD).
2. Результат preflight: утечки не обнаружены.
3. Секреты не ротировались в рамках cutover по подтвержденному решению владельца (`risk-accepted`).

## Required secrets
1. `DB_DSN_FILE`
2. `INVITE_HASH_PEPPER_FILE`
3. `TELEGRAM_BOT_TOKEN_FILE`
4. `TELEGRAM_WEBHOOK_SECRET_FILE`
5. `MAX_BOT_TOKEN_FILE`
6. `MAX_WEBHOOK_SECRET_FILE`
7. `BACKUP_ENCRYPTION_KEY_FILE`

## Edge/network
1. Публикуется только Nginx (443).
2. Webhook endpoints: POST only.
3. Проверка secret headers для Telegram/MAX.
4. `client_max_body_size`, `limit_req`, read/write timeouts.
5. PostgreSQL не публикуется наружу.

## Host baseline
1. Отдельный непривилегированный user.
2. SSH только по ключам, root login disabled.
3. Firewall: только 22/443.
4. Автообновления security patches.

## Data minimization
1. Не хранить payload неавторизованных MAX-сообщений.
2. Не хранить invalid invite attempts.
3. Invite коды в БД только hash.
4. Raw токены и raw invite values в лог не попадают.
