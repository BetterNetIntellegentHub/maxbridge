# Operations Runbook (Public)

## 1. Purpose

This document keeps a public-safe operational baseline.
Detailed environment-specific procedures must remain in private ops docs.

## 2. Local operations

1. Prepare local secret files (see `deploy/compose/secrets/README.md`).
2. Start stack: `docker compose -f deploy/compose/docker-compose.yml up -d`.
3. Apply migrations: `docker compose -f deploy/compose/docker-compose.yml exec bridge /app/bridge migrate up`.
4. Check health endpoints (`/health/live`, `/health/ready`, `/health/checks`).
5. Open TUI with `./scripts/maxbridge`.

## 3. Production model (high level)

1. CI/CD source of truth: GitHub Actions workflows in `.github/workflows/`.
2. Deploy path: immutable container image + Ansible deploy/rollback.
3. Secrets are expected from secure runtime sources (GitHub Environments and/or private vault workflow).
4. `main` must stay protected with required CI checks.
5. Detailed runner topology, host-level service operations, and emergency procedures are private.

## 4. Retention and queue guarantees

1. `delivery_jobs` and `delivery_attempts` cleanup follows TTL policy.
2. `dedupe_records` cleanup follows expiration policy.
3. Worker recovers stale leased jobs and requeues safely.

## 5. Backup/restore model

1. Backups are encrypted and scheduled.
2. Restore is validated regularly in non-production environment.
3. Detailed backup storage and restoration operating procedures are private.

## 6. Public safety notes

1. Do not publish real infra identifiers, host paths, runner-local paths, or internal recovery details.
2. Keep this document synchronized with `docs/project-context.md`.
3. Store extended operational runbooks outside this public repository.
