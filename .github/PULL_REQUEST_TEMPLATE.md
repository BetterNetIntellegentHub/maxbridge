## Summary

<!-- What changed and why -->

## Risk / Impact

- [ ] No runtime behavior change
- [ ] Runtime behavior change is intentional and documented
- [ ] Rollback path is clear

## Public-repo security checklist (required)

- [ ] I did not commit secrets, tokens, keys, `.env` with real values, inventory prod files, dumps, or backups.
- [ ] Any config examples added/updated are template-only and redacted.
- [ ] Documentation changes are sanitized for a public repo (no internal hostnames/paths/runner-local details).
- [ ] Build/release artifacts introduced by this PR do not contain sensitive values.

## High-sensitivity changes

- [ ] CI/CD workflow change (`.github/workflows/*`)
- [ ] Deploy/rollback/infra change (`deploy/*`, deploy scripts)
- [ ] Secret management change
- [ ] Backup/restore change
- [ ] Security policy/docs change

## Validation

<!-- List exact commands/checks run -->

## Manual follow-ups (outside Git)

<!-- Required owner actions, if any -->
