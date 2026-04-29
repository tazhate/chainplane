#!/usr/bin/env bash
# Adds the SPDX/Apache-2.0 header to every Go source file under api/, cmd/, internal/.
# Idempotent: skips files that already contain "SPDX-License-Identifier".
# Skips kubebuilder-generated files (zz_generated.*) since they have their own
# header derived from hack/boilerplate.go.txt.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HEADER_FILE="${REPO_ROOT}/hack/boilerplate.go.txt"

if [[ ! -f "${HEADER_FILE}" ]]; then
    echo "ERROR: ${HEADER_FILE} not found" >&2
    exit 1
fi

added=0
skipped=0

while IFS= read -r -d '' file; do
    base="$(basename "${file}")"
    # kubebuilder regenerates these from boilerplate.go.txt automatically.
    if [[ "${base}" == zz_generated.* ]]; then
        skipped=$((skipped + 1))
        continue
    fi
    if grep -q "SPDX-License-Identifier" "${file}"; then
        skipped=$((skipped + 1))
        continue
    fi
    tmp="$(mktemp)"
    cat "${HEADER_FILE}" "${file}" > "${tmp}"
    mv "${tmp}" "${file}"
    added=$((added + 1))
done < <(find "${REPO_ROOT}/api" "${REPO_ROOT}/cmd" "${REPO_ROOT}/internal" \
    -type f -name '*.go' -print0 2>/dev/null)

echo "Headers added:   ${added}"
echo "Files skipped:   ${skipped}"
