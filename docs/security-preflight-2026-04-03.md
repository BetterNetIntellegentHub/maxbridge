# Security Preflight Report (2026-04-03)

## Scope
Public cutover preflight before switching repository visibility to `public`.

## Commands
```bash
go run github.com/zricethezav/gitleaks/v8@v8.24.2 git . --redact --no-banner --report-format sarif --report-path /tmp/maxbridge-gitleaks-history.sarif
go run github.com/zricethezav/gitleaks/v8@v8.24.2 dir . --redact --no-banner --report-format sarif --report-path /tmp/maxbridge-gitleaks-head.sarif
```

## Result
1. Full history scan: no leaks found.
2. HEAD scan: no leaks found.
3. Findings classification: no true positives, no false positives.

## Risk note
Secrets were not rotated during this cutover by explicit owner decision (`risk-accepted`).
