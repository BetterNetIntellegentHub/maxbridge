# SourceCraft Free Migration Runbook

## 1. Create SourceCraft project and import repository
1. Create a private SourceCraft repository.
2. Import current repository history from GitHub/GitLab mirror.
3. Set `main` as the default branch.

## 2. Governance (merge gate)
1. Commit `.sourcecraft/branches.yaml` and `.sourcecraft/review.yaml` into `main`.
2. In SourceCraft UI, enable policy that merge to `main` requires successful CI.
3. Verify direct push to `main` is blocked and PR approvals are required.

## 3. Secrets and variables
1. Create shared secrets/variables:
   - `REGISTRY_USERNAME`
   - `REGISTRY_PASSWORD`
   - `MAXBRIDGE_IMAGE_REPO`
2. Create environment-scoped values for `staging` and `production`:
   - `MAXBRIDGE_TELEGRAM_BOT_TOKEN`
   - `MAXBRIDGE_MAX_BOT_TOKEN`
   - `MAXBRIDGE_REGISTRY_TOKEN`
   - `MAXBRIDGE_DOMAIN`
   - `MAXBRIDGE_HTTPS_PORT`
   - `DEPLOY_HOST`
   - `DEPLOY_USER`
   - `DEPLOY_SSH_KEY_PATH`
   - `DEPLOY_SSH_KNOWN_HOSTS_PATH`
   - `ALLOWED_PROD_USER`
3. Mark all sensitive values as protected/hidden in SourceCraft.

## 4. Register WSL self-hosted worker
1. Install SourceCraft worker on WSL host per official docs.
2. Register worker for this repository with tag `wsl-deploy`.
3. Ensure service auto-start via systemd (`sourcecraft-worker.service` or installed service name).
4. Validate worker online in SourceCraft UI.

## 5. Phased cutover
1. **Phase 1 (CI parity):** run `ci-checks` on PR and `main` push.
2. **Phase 2 (image parity):** run `image-publish`, verify pushed tags + SBOM + Trivy pass.
3. **Phase 3 (staging):** manually run `deploy-staging`, then `rollback-staging`.
4. **Phase 4 (production):** run `deploy-production` with `PRODUCTION_CONFIRM=DEPLOY_PRODUCTION`.
5. **Phase 5 (decommission old CI):** keep GitHub/GitLab as read-only mirrors and disable their CI/CD flows.

## 6. Acceptance checks
1. `main` rejects direct push.
2. PR without approval is blocked.
3. PR with failed `ci-checks` is blocked.
4. Deploy/rollback workflows are routed only to `wsl-deploy` worker.
5. Post-check succeeds: `/health/ready` and `/health/checks` queue fields are valid.
