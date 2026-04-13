# Public Repository Policy

Updated: 2026-04-03

## 1. Scope

This policy defines what can and cannot be stored in the public MaxBridge repository.

## 2. Never commit

1. Real secrets (tokens, API keys, SSH keys, cert/private key material, backup keys).
2. `.env` files with real values.
3. Production inventory/host details.
4. Database dumps, encrypted backups, restore input artifacts.
5. Debug dumps containing operational data.

## 3. Documentation hygiene

1. Public docs must stay sanitized.
2. Do not publish real hostnames/domains, internal paths, runner-local paths, or environment-specific infrastructure details.
3. Do not publish detailed operational recovery nuances that are not required for external readers.
4. Keep private operational details in private ops documentation.

## 4. CI/CD and workflow hygiene

1. Workflows must use least-privilege permissions.
2. Use pinned action versions and avoid risky installer patterns.
3. Any change in CI/CD, deploy, secrets, backup/restore, or security docs requires elevated review attention.
4. Files resembling secrets or forbidden artifacts must be blocked before merge.

## 5. Change management requirements

1. Keep changes minimal, reviewable, and reversible.
2. Document manual follow-ups when a fix cannot be fully enforced in Git.
3. Preserve working deploy path unless a confirmed security issue requires change.

## 6. Public cutover follow-ups (manual, outside Git)

1. Maintain GitHub Environments protections compatible with the automated delivery flow.
2. Keep emergency rollback access controls and runner permissions least-privileged.
3. Keep extended operational runbooks outside this public repo.
4. Reassess secret rotation posture periodically after public cutover.
5. Keep branch protection required checks in sync with current CI workflow.
