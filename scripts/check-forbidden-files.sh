#!/usr/bin/env bash
set -euo pipefail

# Block files that should never be tracked in this public repository.
mapfile -t tracked < <(git ls-files)

patterns=(
  '^\.env$'
  '^\.env\..+'
  '^deploy/ansible/inventory/hosts\.yml$'
  '^deploy/ansible/group_vars/all/vault\.yml$'
  '^.*\.pem$'
  '^.*\.key$'
  '^.*\.crt$'
  '^.*\.p12$'
  '^.*\.kdbx$'
  '^.*\.enc$'
  '^.*\.dump$'
  '^.*\.bak$'
  '^.*\.backup$'
  '^.*\.dmp$'
  '^.*\.core$'
  '^backup/.+'
  '^restore/.+'
  '^.*id_rsa$'
  '^.*id_ed25519$'
)

allow_patterns=(
  '^\.env\.example$'
  '^deploy/compose/\.env\.example$'
  '^deploy/compose/secrets/examples/.+\.example$'
)

violations=()

is_allowed() {
  local path="$1"
  for allow in "${allow_patterns[@]}"; do
    if [[ "$path" =~ $allow ]]; then
      return 0
    fi
  done
  return 1
}

for path in "${tracked[@]}"; do
  if is_allowed "$path"; then
    continue
  fi

  for pattern in "${patterns[@]}"; do
    if [[ "$path" =~ $pattern ]]; then
      violations+=("$path")
      break
    fi
  done

done

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "Forbidden tracked files detected:" >&2
  for v in "${violations[@]}"; do
    echo "  - $v" >&2
  done
  echo "Remove or sanitize these files before merge." >&2
  exit 1
fi

echo "No forbidden tracked files detected."
