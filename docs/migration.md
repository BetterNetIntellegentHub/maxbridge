# Migration & Rollback

## Database migration strategy
1. Используются SQL миграции `migrations/*.up.sql` и `*.down.sql`.
2. Команда:
   - `bridge migrate up`
   - `bridge migrate down`
3. Стратегия: backward-compatible изменения (expand/contract).

## Release sequence
1. Deploy image N+1.
2. Run `migrate up`.
3. Start services.
4. Verify health/metrics.

## Rollback strategy
1. Откат приложения на previous immutable image tag.
2. Down migrations применять только если они безопасны для фактических данных.
3. Предпочтительный rollback при инциденте: app rollback + feature flag/route pause.

## Server migration
1. Новый сервер bootstrap Ansible.
2. Restore PostgreSQL backup.
3. Deploy compose stack и image tags.
4. Health checks.
5. Переключение DNS.
