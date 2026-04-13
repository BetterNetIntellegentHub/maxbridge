# Security Notes (Public)

## 1. Secrets and sensitive data

1. Secrets must not be stored in git-tracked files.
2. Runtime uses file-based secrets (`*_FILE`) and secure secret sources.
3. Sensitive fields in logs must remain redacted (`token`, `secret`, `password`, `invite`).
4. Public docs/examples must be template-only and sanitized.

## 2. Public cutover status

1. Public cutover preflight used gitleaks scans (`git` and `dir`) and reported no findings.
2. A previous owner decision accepted risk of not rotating secrets at cutover time (`risk-accepted`).
3. Secret rotation posture should be periodically reassessed outside Git.

## 3. Required operational controls

1. Enforce webhook secret headers for Telegram/MAX.
2. Keep edge exposure minimal (only required public endpoints).
3. Keep PostgreSQL non-public.
4. Preserve data-minimization behavior:
   - invalid/unauthorized MAX traffic: no reply + no DB write;
   - invalid invite attempts are ignored;
   - invite raw values are not logged.

## 4. Public repo process controls

1. Use `SECURITY.md` for disclosure policy.
2. Use `docs/public-repo-policy.md` for hygiene and publication rules.
3. Keep extended operational security runbooks in private documentation.

## 5. Manual follow-ups (outside Git)

1. Maintain GitHub Environments protections compatible with automated deployment flow.
2. Keep required CI checks aligned with branch protection.
3. Periodically review cutover-era secrets and decide on rotation.
4. Keep manual rollback controls and access restrictions for emergency operations.
