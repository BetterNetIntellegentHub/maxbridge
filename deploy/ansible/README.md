# Ansible usage

## Bootstrap

```bash
ansible-playbook -i inventory/hosts.yml bootstrap.yml
```

Create local inventory from template first:

```bash
cp inventory/hosts.example.yml inventory/hosts.yml
```

## Deploy

```bash
ansible-playbook -i inventory/hosts.yml deploy.yml -e "maxbridge_version=<tag>" -e "maxbridge_domain=<domain>"
```

## Recommended secrets flow (no manual server console)

1. Keep `maxbridge_manage_secrets: true` (default in `group_vars/all/base.yml`).
2. Put external bot tokens into encrypted Ansible vars (Vault), for example:

```yaml
# group_vars/all/vault.yml (encrypted with ansible-vault)
maxbridge_telegram_bot_token: "<telegram_token>"
maxbridge_max_bot_token: "<max_token>"
maxbridge_registry_token: "<docker_hub_access_token>"
```

3. Run deploy with Vault password:

```bash
ansible-playbook -i inventory/hosts.yml deploy.yml \
  --ask-vault-pass \
  -e "maxbridge_version=<tag>" \
  -e "maxbridge_domain=<domain>"
```

With this mode, Ansible creates/maintains secret files under `{{ maxbridge_secrets_dir }}` and syncs them to compose automatically.

## Private Docker Hub

Use this flow if `docker.io/<user>/<repo>` is private.

1. Configure non-secret registry vars (for example in `group_vars/all/base.yml`):

```yaml
maxbridge_registry_private: true
maxbridge_registry_url: "https://index.docker.io/v1/"
maxbridge_registry_username: "argusvlad"
```

2. Store registry token in Vault (`group_vars/all/vault.yml`):

```yaml
maxbridge_registry_token: "<docker_hub_access_token>"
```

3. Run deploy with Vault password:

```bash
ansible-playbook -i inventory/hosts.yml deploy.yml \
  --ask-vault-pass \
  -e "maxbridge_image=docker.io/<user>/maxbridge:<tag>" \
  -e "maxbridge_version=<tag>" \
  -e "maxbridge_domain=<domain>"
```

When `maxbridge_registry_private=true`, deploy fails early with a clear error if `maxbridge_registry_username` or `maxbridge_registry_token` is missing.

## Port note (when 443 is occupied)

If another service already uses `443`, deploy MaxBridge on another HTTPS host port (for example `8443`):

```bash
ansible-playbook -i inventory/hosts.yml deploy.yml \
  --ask-vault-pass \
  -e "maxbridge_version=<tag>" \
  -e "maxbridge_domain=<domain>" \
  -e "maxbridge_https_port=8443" \
  -e "maxbridge_healthcheck_url=https://<domain>:8443/health/ready"
```

## Requirements

1. `community.docker` collection installed.
2. SSH key-based access for deploy user.
3. If `maxbridge_manage_secrets: false`, prepare files in path from `maxbridge_secrets_src`.
4. `group_vars/all/vault.yml` and `inventory/hosts.yml` are local operational files and should stay untracked in git.
