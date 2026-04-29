#!/usr/bin/env bash
# Migrates the chainplane API surface from v1alpha2/ChainInstance to v1alpha2/ChainInstance.
#
# Renames performed (order matters - longest patterns first):
#   group:           chains.chainplane.io        ->  chains.chainplane.io
#   plural resource: chaininstances            ->  chaininstances
#   singular:        chaininstance             ->  chaininstance
#   Go alias import: chainsv1alpha2              ->  chainsv1alpha2
#   Go type:         ChainInstanceReconciler   ->  ChainInstanceReconciler
#                    ChainInstanceSpec         ->  ChainInstanceSpec
#                    ChainInstanceStatus       ->  ChainInstanceStatus
#                    ChainInstanceList         ->  ChainInstanceList
#                    ChainInstancePhase        ->  ChainInstancePhase
#                    ChainInstanceWebhook      ->  ChainInstanceWebhook
#                    ChainInstanceValidator    ->  ChainInstanceValidator
#                    ChainInstanceDefaulter    ->  ChainInstanceDefaulter
#                    ChainInstance             ->  ChainInstance
#   Package path:    api/v1alpha2               ->  api/v1alpha2
#
# NOTE: existing `type Chain string` enum (chain protocol identifiers like
# bitcoin/ethereum) is preserved untouched. Renaming the CRD to ChainInstance
# avoids collision with the Chain enum.
#
# Idempotent. Run multiple times if needed; subsequent runs are no-ops.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

mapfile -t FILES < <(find api cmd internal config charts docs web hack \
    -type f \
    \( -name '*.go' -o -name '*.yaml' -o -name '*.yml' -o -name '*.md' \
       -o -name '*.html' -o -name '*.json' -o -name '*.sh' -o -name '*.toml' \
       -o -name '*.tmpl' -o -name 'Dockerfile*' -o -name 'Makefile' \) \
    -not -path './bin/*' \
    -not -path './charts/*.tgz' \
    2>/dev/null)

for top in README.md CHANGELOG.md CONTRIBUTING.md QUICKSTART.md PROJECT Makefile go.mod go.sum; do
    [[ -f "${top}" ]] && FILES+=("${top}")
done

SED_EXPR=$(cat <<'EOF'
s|nodes\.chainplane\.io|chains.chainplane.io|g
s|chainsv1alpha2|chainsv1alpha2|g
s|ChainInstanceReconciler|ChainInstanceReconciler|g
s|ChainInstanceSpec|ChainInstanceSpec|g
s|ChainInstanceStatus|ChainInstanceStatus|g
s|ChainInstanceList|ChainInstanceList|g
s|ChainInstancePhase|ChainInstancePhase|g
s|ChainInstanceWebhook|ChainInstanceWebhook|g
s|ChainInstanceValidator|ChainInstanceValidator|g
s|ChainInstanceDefaulter|ChainInstanceDefaulter|g
s|ChainInstance|ChainInstance|g
s|chaininstances|chaininstances|g
s|chaininstance|chaininstance|g
s|"github\.com/tazhate/chainplane/api/v1alpha2"|"github.com/tazhate/chainplane/api/v1alpha2"|g
s|api/v1alpha2|api/v1alpha2|g
EOF
)

count=0
for file in "${FILES[@]}"; do
    [[ -f "${file}" ]] || continue
    if grep -qE 'ChainInstance|chaininstance|chainsv1alpha2|nodes\.chainplane\.io|api/v1alpha2' "${file}" 2>/dev/null; then
        sed -i "${SED_EXPR}" "${file}"
        count=$((count + 1))
    fi
done

echo "Files rewritten: ${count}"
