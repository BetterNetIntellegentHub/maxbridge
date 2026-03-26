# Архитектура Telegram -> MAX Bridge

## Статус
Принято.

## Контекст
Нужен надёжный сервис-мост с TUI-управлением, без web-admin, с durable delivery и минимизацией данных.

## Решение
1. **Modular monolith + worker process**:
   - `bridge` (ingress/webhooks, health, metrics, housekeeping)
   - `worker` (delivery queue processing)
   - `tui` (операторский fullscreen интерфейс)
2. **PostgreSQL как durable queue**:
   - `delivery_jobs` для жизненного цикла доставки
   - `delivery_attempts` (partitioned) для попыток
   - `dedupe_records` для идемпотентности
3. **Webhook-first ingress**:
   - Telegram и MAX принимаются через HTTPS reverse proxy
   - валидация secret headers и быстрый ACK
   - тяжёлая доставка только в worker
4. **Invite-based onboarding**:
   - raw invite code показывается только при создании
   - в БД только hash кода
   - invalid/unauthorized MAX traffic не сохраняется
5. **GitOps-lite deployment**:
   - Docker Compose runtime
   - Ansible provisioning/deploy
   - immutable image tags, rollback через предыдущий tag

## Ограничения и явные решения по документации
1. MAX: для production используется webhook; long polling только dev/debug.
2. Telegram privacy mode: если `can_read_all_group_messages=false`, группа помечается `LIMITED`.
3. Telegram deep linking `startgroup` используется как операторский helper и не гарантирует readiness без probe.
4. Если требование противоречит docs, приоритет у официальной документации; фиксируется в `docs/NOTES.md`.

## Последствия
1. Получаем at-least-once delivery и crash recovery без внешнего брокера.
2. Возможна повторная доставка в edge-cases, но логически дубликаты ограничиваются dedupe key.
3. База контролируемо очищается retention-процедурами.
