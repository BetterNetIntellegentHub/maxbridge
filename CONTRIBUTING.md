# Contributing to MaxBridge

## Public repository rules

1. This repository is public. Treat all commits, PR comments, and CI logs as publicly visible.
2. Never commit:
   - real secrets, tokens, keys, certificates, SSH material;
   - real `.env` files;
   - production inventory/host details;
   - database dumps, backup artifacts, restore inputs;
   - debug dumps containing operational data.
3. Configuration examples must be templated and redacted.
4. Detailed private operational procedures must stay in private ops docs.

## Change scope expectations

1. Prefer minimal, reviewable, reversible changes.
2. Avoid broad refactors when solving a narrow issue.
3. Do not change runtime behavior unless required by a confirmed issue.
4. For risky changes, document rollback expectations in the PR.

## Security-sensitive areas (extra review required)

1. CI/CD workflows under `.github/workflows/`.
2. Deploy/rollback and infra (`deploy/`, `scripts/deploy.sh`, `scripts/rollback.sh`).
3. Secret management (`deploy/compose/secrets`, Ansible vars, security docs).
4. Backup/restore flows.
5. Documentation that may expose infrastructure or operational details.

## Required checks

Before opening/updating a PR, run as applicable:

```bash
go test ./...
```

Also ensure CI checks pass (including secret scanning and repository hygiene checks).

## Pull request checklist

Use `.github/PULL_REQUEST_TEMPLATE.md` and confirm:
1. no secrets or prohibited files are introduced;
2. docs remain public-safe and sanitized;
3. changes are minimal and reversible;
4. follow-up manual actions are explicitly documented when needed.

## Disclosure and incidents

1. Do not open public issues for suspected secrets exposure.
2. Follow `SECURITY.md` for private disclosure workflow.
