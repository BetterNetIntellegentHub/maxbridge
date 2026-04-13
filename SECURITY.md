# Security Policy

## Supported security posture

MaxBridge repository is public and follows strict public-repo hygiene:
1. no real secrets in git history or tracked files;
2. sanitized public documentation only;
3. CI secret scanning and repository hygiene checks are required.

## Reporting a vulnerability

1. Do not open a public issue for security vulnerabilities or potential secret exposure.
2. Report privately to repository maintainers using GitHub Security Advisories (preferred) or other private maintainer contact channel.
3. Include:
   - affected component/path;
   - impact assessment;
   - minimal reproduction or evidence;
   - suggested remediation if available.

## Sensitive data handling rules

1. Never commit real tokens, API keys, SSH keys, certificates, backup keys, inventory files with real hosts, or `.env` files with real values.
2. Never include sensitive values in PR descriptions, issue comments, workflow logs, or screenshots.
3. Keep operational/private details in private ops documentation; public docs must remain sanitized.
4. Use template/example files for configuration.

## Public cutover process gap tracking

Current repository controls include CI secret scanning and public-doc sanitization policy.

Manual follow-ups outside Git are still required for secure operations:
1. maintain and review GitHub Environments protection rules and secrets ownership;
2. set and maintain `MAXBRIDGE_PROD_DEPLOY_ACTOR` in GitHub Variables/Environments for production deploy/rollback allowlist;
3. maintain private operational runbooks separately from this public repo;
4. periodically reassess the previous `risk-accepted` decision regarding non-rotated cutover secrets;
5. keep branch protection required checks aligned with current CI jobs.

## Security-related docs

1. `docs/security.md`
2. `docs/public-repo-policy.md`
3. `AGENTS.md`
4. `CONTRIBUTING.md`
