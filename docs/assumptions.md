# Assumptions

1. Runtime: Linux LTS single server, Docker Engine + Docker Compose plugin.
2. PostgreSQL 16+ доступен в отдельном контейнере внутри приватной сети compose.
3. MAX API base URL в production: `https://botapi.max.ru`.
4. Invite scopes:
   - `group`: `scope_id` = telegram chat id (int64)
   - `route`: `scope_id` = existing route id
   - `entity`: общий кастомный scope, без автоматического route-bind.
5. Default retry policy: max 8 attempts, exponential backoff with jitter.
6. Retention defaults:
   - jobs/attempts: 30 days
   - dedupe records: 14 days
   - payload wipe after successful delivery: 24 hours
7. Нет Kubernetes и внешнего брокера сообщений в v1.
