# Agent Operating Rules for MaxBridge

Updated: 2026-04-03
Owner: Codex

## 1. Главные принципы

1. Безопасность и надёжность важнее скорости.
2. Никаких silent-breaking изменений в очереди доставки, invite-flow и webhook-валидации.
3. Любое изменение должно быть эксплуатационно объяснимым и воспроизводимым.
4. Источник контекста по проекту: `docs/project-context.md`.
5. Репозиторий публичный: любые изменения должны учитывать public-by-default модель.

## 2. Порядок источников истины

При противоречии использовать приоритет:
1. Код в репозитории.
2. `docs/project-context.md`.
3. ADR и runbook документы (`docs/adr/*`, `docs/operations.md`, `docs/security.md`).
4. README.

Если `docs/project-context.md` расходится с кодом:
1. Сначала подтвердить фактическое поведение в коде.
2. Затем обновить `docs/project-context.md` в том же change set.

## 3. Перед началом любой задачи

1. Прочитать:
   - `docs/project-context.md`
   - релевантный кодовый модуль
   - связанный runbook/ADR при необходимости
2. Уточнить влияние на:
   - delivery durability
   - data minimization
   - security boundaries
   - deploy/rollback behavior
3. Сформировать мини-план изменений (что, где, как проверить).

## 4. Правила изменений кода

1. Не ломать обязательные инварианты:
   - invalid/unauthorized MAX traffic: no reply + no DB write
   - webhook processing: quick ACK, no heavy inline delivery
   - queue: at-least-once + dedupe + retry + DLQ + lease recovery
2. Не вносить secrets в код, env примеры, логи, docs.
3. Не удалять/ослаблять проверку webhook secret headers.
4. Не увеличивать длительное хранение payload без явного основания.
5. При изменении schema:
   - только миграциями
   - backward-compatible подход по умолчанию

## 5. Правила публичного репозитория

1. Запрещено коммитить:
   - реальные токены, ключи, SSH material, backup keys;
   - `.env` с реальными значениями;
   - inventory/host файлы prod;
   - дампы/бэкапы/restore inputs;
   - debug dumps и runtime diagnostic файлы.
2. Публичная документация должна быть sanitized:
   - без реальных hostnames, доменов, внутренних путей и runner-local путей;
   - без чувствительных деталей recovery/ops процедур;
   - детальные операционные инструкции хранятся в private ops docs.
3. Примеры конфигов должны быть шаблонными/redacted.
4. Изменения в CI/CD, deploy, secrets, backup/restore, docs/security считаются high-review-sensitivity.
5. Новые workflows должны соблюдать least privilege и безопасные практики GitHub Actions.

## 6. Правила работы с TUI

1. Все операторские действия должны оставаться возможными без web-admin.
2. Новые TUI-команды обязаны иметь:
   - явный usage
   - безопасное поведение по умолчанию
   - отсутствие утечки секретов в выводе
3. Для destructive-операций в TUI добавлять явный guard/подтверждение (если применимо).

## 7. Правила по БД и очереди

1. Любые изменения в `delivery_jobs`, `delivery_attempts`, `dedupe_records` проверять на:
   - конкуренцию
   - idempotency
   - восстановление после crash
2. Сохранять индексы, критичные для claim/retry/cleanup.
3. Не менять TTL-политику без обновления `docs/operations.md`.

## 8. Правила по инфраструктуре

1. Runtime модель: single-server Docker Compose + Nginx + PostgreSQL.
2. Provision/deploy: только через Ansible (без snowflake ручных правок как обязательного шага).
3. Основной delivery flow должен оставаться автоматизированным:
   - `ci` -> `cd-image` -> `cd-deploy` (staging -> production),
   - при провале production-checks должен сохраняться автоматический rollback.
4. Любые изменения deploy flow должны включать:
   - обновление Ansible/Compose
   - проверку rollback пути
   - обновление docs
5. CI/CD source of truth: GitHub Actions; для `main` обязателен branch protection + required CI checks.
6. Релизные бинарники (`bridge`, `worker`, `tui`) публиковать через GitHub Releases; не хранить их в git-tracked файлах.

## 9. Проверка перед завершением задачи

1. Обновить релевантные docs:
   - минимум `docs/project-context.md` при архитектурных/операционных изменениях
2. Проверить, что не добавлены секреты в git-tracked файлы.
3. Проверить, что acceptance-инварианты не нарушены.
4. Проверить, что не добавлены запрещённые файлы (env/secrets/backup/dump/inventory).
5. Выполнить безопасную post-task очистку временных артефактов, если они создавались.
6. Явно указать ограничения верификации (если нет runtime/инструментов).

## 10. Формат отчёта пользователю

1. Что изменено (кратко, по подсистемам).
2. Что проверено и чем.
3. Что не удалось проверить и почему.
4. Что нужно сделать пользователю дальше (только если действительно нужно).

## 11. Запрещённые действия

1. Логировать raw tokens/invite values или webhook payload с чувствительными данными.
2. Добавлять автосохранение invalid link attempts в БД.
3. Переносить heavy delivery логику внутрь webhook handler.
4. Размывать границы доступа к БД наружу через edge.

## 12. Обязательное обновление этого файла

Обновлять этот файл при:
1. изменении базовой архитектуры;
2. изменении инвариантов безопасности/очереди;
3. изменении deploy/rollback/restore процедуры;
4. изменении операторской модели TUI;
5. изменении политики публичного репозитория.
