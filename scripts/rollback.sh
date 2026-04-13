#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: rollback.sh <inventory> <previous_version_tag> <domain>" >&2
  exit 1
fi

INVENTORY="$1"
VERSION="$2"
DOMAIN="$3"

ansible-playbook -i "$INVENTORY" deploy/ansible/deploy.yml \
  -e "maxbridge_version=$VERSION" \
  -e "maxbridge_domain=$DOMAIN"

