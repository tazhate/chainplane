#!/usr/bin/env bash
# Renames the embedded-inheritance adapter base types throughout
# internal/adapters/ to make naming distinct from any prior project
# in the same problem space.
#
#   baseAdapter      ->  protocolAdapter
#   utxoAdapter      ->  utxoProtocolAdapter
#
# Idempotent.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}/internal/adapters"

count=0
for file in *.go; do
    [[ -f "${file}" ]] || continue
    if grep -qE 'baseAdapter|utxoAdapter' "${file}" 2>/dev/null; then
        # Order matters: longer pattern first.
        sed -i -e 's|utxoAdapter|utxoProtocolAdapter|g' \
               -e 's|baseAdapter|protocolAdapter|g' "${file}"
        count=$((count + 1))
    fi
done

# Rename the files that define the base types.
[[ -f base.go ]] && mv base.go protocol_base.go
[[ -f utxo.go ]] && mv utxo.go utxo_protocol_base.go

echo "Adapter files rewritten: ${count}"
