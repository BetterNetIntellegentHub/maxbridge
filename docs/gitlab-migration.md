# GitLab Free Migration Runbook

## 1. Create GitLab project and import repository
1. Create private project in GitLab.com.
2. Import from GitHub repository `BetterNetIntellegentHub/maxbridge`.
3. Set default branch to `main`.

## 2. Governance (merge gate)
1. Protect branch `main`:
   - direct push: disabled;
   - merge: Maintainer only.
2. Enable "Pipelines must succeed".
3. Enable merge settings:
   - squash on merge;
   - delete source branch after merge;
   - do not allow merge without pipeline.

## 3. Variables and secrets (GitLab CI/CD)
1. Add protected/shared variables:
   - `REGISTRY_USERNAME`
   - `REGISTRY_PASSWORD`
   - `MAXBRIDGE_IMAGE_REPO`
2. Add environment-scoped variables for `staging` and `production`:
   - `MAXBRIDGE_TELEGRAM_BOT_TOKEN`
   - `MAXBRIDGE_MAX_BOT_TOKEN`
   - `MAXBRIDGE_REGISTRY_TOKEN`
   - `MAXBRIDGE_DOMAIN`
   - `MAXBRIDGE_HTTPS_PORT`
   - `DEPLOY_HOST`
   - `DEPLOY_USER`
   - `DEPLOY_SSH_KEY_PATH`
   - `DEPLOY_SSH_KNOWN_HOSTS_PATH`
3. Mark sensitive values as masked + protected.

## 4. Register WSL runner
1. Install GitLab Runner in WSL (Ubuntu):
   - `curl -L --output gitlab-runner.deb https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner_amd64.deb`
   - `sudo dpkg -i gitlab-runner.deb`
2. Register runner as shell executor:
   - `sudo gitlab-runner register`
   - URL: `https://gitlab.com/`
   - token: project/group runner token from GitLab UI
   - executor: `shell`
   - tags: `wsl-deploy`
3. Enable service autostart:
   - `sudo systemctl enable --now gitlab-runner`
4. Validate:
   - `sudo gitlab-runner verify`
   - `systemctl status gitlab-runner`

## 5. Cutover
1. Run GitLab pipeline in `staging`:
   - `image_publish` -> `deploy_staging` -> `rollback_staging`.
2. Run production manual path:
   - `deploy_production` with `PRODUCTION_CONFIRM=DEPLOY_PRODUCTION`.
3. After successful cutover:
   - remove CD secrets from GitHub;
   - keep GitHub as read-only mirror only.

## 6. Post-cutover checks
1. `main` rejects direct push and requires green pipeline for merge.
2. Deploy/rollback jobs run only on runner tag `wsl-deploy`.
3. `health/ready` and `health/checks` post-checks pass.
4. Pipeline logs do not expose masked secrets.
