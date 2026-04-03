# Ansible Usage (Public)

## Scope

This is a public-safe Ansible usage guide.
Detailed environment-specific inventory, host operations, and private recovery details belong to private ops docs.

## Bootstrap

```bash
cp inventory/hosts.example.yml inventory/hosts.yml
ansible-playbook -i inventory/hosts.yml bootstrap.yml
```

## Deploy

```bash
ansible-playbook -i inventory/hosts.yml deploy.yml \
  -e "maxbridge_version=<tag>" \
  -e "maxbridge_domain=<public_domain>"
```

## Rollback

```bash
ansible-playbook -i inventory/hosts.yml deploy.yml \
  -e "maxbridge_version=<previous_tag>" \
  -e "maxbridge_domain=<public_domain>"
```

## Secrets handling

1. Prefer secure runtime secret injection (GitHub Environments or private vault flow).
2. Keep `maxbridge_manage_secrets: true` unless private runbook explicitly requires another mode.
3. Do not commit real `inventory/hosts.yml` or `group_vars/all/vault.yml`.

## Notes

1. `inventory/hosts.yml` is local operational data and must remain untracked.
2. Use template/redacted values only in tracked files.
