#!/usr/bin/env bash
# Manual batch e2e: applies all ChainInstance samples in batches, verifies
# pod start + .status.height > 0, then deletes the batch before the next.
#
# Env:
#   BATCH_SIZE      (default 5)
#   READY_TIMEOUT   (default 600 sec per batch)
#   STORAGE_CLASS   (default "standard" — kind default)
#   STORAGE_SIZE    (default 5Gi)
#   NAMESPACE       (default chainplane-batch)
#   CHAINS_FILTER   (regex, empty = all)
#   ARTIFACTS_DIR   (default ./artifacts)
#   SAMPLES_DIR     (default config/samples)

set -uo pipefail

BATCH_SIZE="${BATCH_SIZE:-5}"
READY_TIMEOUT="${READY_TIMEOUT:-600}"
STORAGE_CLASS="${STORAGE_CLASS:-standard}"
STORAGE_SIZE="${STORAGE_SIZE:-5Gi}"
NAMESPACE="${NAMESPACE:-chainplane-batch}"
CHAINS_FILTER="${CHAINS_FILTER:-}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-artifacts}"
SAMPLES_DIR="${SAMPLES_DIR:-config/samples}"

mkdir -p "${ARTIFACTS_DIR}"

for bin in kubectl yq jq; do
  command -v "${bin}" >/dev/null 2>&1 || { echo "ERROR: ${bin} not in PATH"; exit 2; }
done

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Build sample list (skip the placeholder aggregate file).
mapfile -t ALL_SAMPLES < <(
  find "${SAMPLES_DIR}" -maxdepth 1 -name 'chains_v1alpha2_chaininstance_*.yaml' \
    ! -name 'chains_v1alpha2_chaininstance.yaml' | sort
)

if [[ -n "${CHAINS_FILTER}" ]]; then
  FILTERED=()
  for f in "${ALL_SAMPLES[@]}"; do
    [[ "$(basename "$f")" =~ ${CHAINS_FILTER} ]] && FILTERED+=("$f")
  done
  ALL_SAMPLES=("${FILTERED[@]}")
fi

TOTAL=${#ALL_SAMPLES[@]}
if [[ ${TOTAL} -eq 0 ]]; then
  echo "No samples matched filter '${CHAINS_FILTER}'"
  exit 1
fi

echo "Discovered ${TOTAL} sample(s); batch size=${BATCH_SIZE}; per-batch timeout=${READY_TIMEOUT}s"

declare -a RESULTS_NAME RESULTS_STATUS RESULTS_REASON

patch_sample() {
  local in="$1" out="$2"
  yq eval "
    .metadata.namespace = \"${NAMESPACE}\" |
    .spec.storage.size = \"${STORAGE_SIZE}\" |
    .spec.storage.storageClass = \"${STORAGE_CLASS}\" |
    del(.spec.resources.limits) |
    .spec.resources.requests.cpu = \"100m\" |
    .spec.resources.requests.memory = \"256Mi\"
  " "${in}" > "${out}"
}

dump_artifacts() {
  local name="$1"
  local logf="${ARTIFACTS_DIR}/${name}.log"
  {
    echo "=== describe chaininstance/${name} ==="
    kubectl -n "${NAMESPACE}" describe chaininstance "${name}" 2>&1 || true
    echo
    echo "=== describe sts ==="
    kubectl -n "${NAMESPACE}" describe sts -l "chains.chainplane.io/instance=${name}" 2>&1 || true
    echo
    echo "=== pod logs ==="
    for p in $(kubectl -n "${NAMESPACE}" get pods -l "chains.chainplane.io/instance=${name}" -o name 2>/dev/null); do
      echo "--- ${p} ---"
      kubectl -n "${NAMESPACE}" logs "${p}" --all-containers --tail=200 2>&1 || true
    done
    echo
    echo "=== events ==="
    kubectl -n "${NAMESPACE}" get events --sort-by=.lastTimestamp 2>&1 | grep -E "${name}|Warning" || true
  } > "${logf}"
}

run_batch() {
  local -a files=("$@")
  local tmpdir
  tmpdir="$(mktemp -d)"
  local -a names=()

  for f in "${files[@]}"; do
    local nm
    nm="$(yq eval '.metadata.name' "$f")"
    names+=("$nm")
    patch_sample "$f" "${tmpdir}/${nm}.yaml"
  done

  echo "--- applying batch: ${names[*]}"
  kubectl apply -f "${tmpdir}/" || true

  local deadline=$(( $(date +%s) + READY_TIMEOUT ))
  local -A done_map=()

  while [[ $(date +%s) -lt ${deadline} ]]; do
    local all_done=1
    for nm in "${names[@]}"; do
      [[ -n "${done_map[$nm]:-}" ]] && continue
      local height phase
      height="$(kubectl -n "${NAMESPACE}" get chaininstance "${nm}" -o jsonpath='{.status.height}' 2>/dev/null || true)"
      phase="$(kubectl -n "${NAMESPACE}" get chaininstance "${nm}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
      if [[ -n "${height}" && "${height}" =~ ^[1-9][0-9]*$ ]]; then
        done_map[$nm]="PASS:height=${height},phase=${phase}"
        echo "    PASS ${nm} height=${height} phase=${phase}"
      else
        all_done=0
      fi
    done
    [[ ${all_done} -eq 1 ]] && break
    sleep 10
  done

  for nm in "${names[@]}"; do
    if [[ -n "${done_map[$nm]:-}" ]]; then
      RESULTS_NAME+=("${nm}")
      RESULTS_STATUS+=("PASS")
      RESULTS_REASON+=("${done_map[$nm]#PASS:}")
    else
      local reason
      reason="$(kubectl -n "${NAMESPACE}" get chaininstance "${nm}" -o jsonpath='phase={.status.phase} height={.status.height}' 2>/dev/null || echo 'no-status')"
      RESULTS_NAME+=("${nm}")
      RESULTS_STATUS+=("FAIL")
      RESULTS_REASON+=("timeout: ${reason}")
      dump_artifacts "${nm}"
      echo "    FAIL ${nm} (${reason})"
    fi
  done

  kubectl delete -f "${tmpdir}/" --wait=false 2>&1 | tail -5 || true
  # Best-effort PVC cleanup (statefulset PVCs aren't auto-deleted)
  kubectl -n "${NAMESPACE}" delete pvc -l app.kubernetes.io/managed-by=chainplane --wait=false 2>/dev/null || true
  rm -rf "${tmpdir}"
}

i=0
while [[ ${i} -lt ${TOTAL} ]]; do
  end=$(( i + BATCH_SIZE ))
  [[ ${end} -gt ${TOTAL} ]] && end=${TOTAL}
  echo
  echo "=== batch $((i / BATCH_SIZE + 1)) [$((i+1))-${end}/${TOTAL}] ==="
  run_batch "${ALL_SAMPLES[@]:i:$((end - i))}"
  i=${end}
done

echo
echo "=== SUMMARY ==="
fail=0
{
  printf "| %-40s | %-6s | %s\n" "chain" "status" "reason"
  printf "|%s|%s|%s\n" "$(printf '%.s-' {1..42})" "$(printf '%.s-' {1..8})" "$(printf '%.s-' {1..40})"
  for n in "${!RESULTS_NAME[@]}"; do
    printf "| %-40s | %-6s | %s\n" "${RESULTS_NAME[$n]}" "${RESULTS_STATUS[$n]}" "${RESULTS_REASON[$n]}"
  done
} > "${ARTIFACTS_DIR}/summary.txt"
cat "${ARTIFACTS_DIR}/summary.txt"

for n in "${!RESULTS_NAME[@]}"; do
  [[ "${RESULTS_STATUS[$n]}" == "FAIL" ]] && fail=$((fail + 1))
done

echo
echo "passed=$(( ${#RESULTS_NAME[@]} - fail )) failed=${fail} total=${#RESULTS_NAME[@]}"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## Manual batch e2e summary"
    echo
    echo "passed=**$(( ${#RESULTS_NAME[@]} - fail ))** failed=**${fail}** total=**${#RESULTS_NAME[@]}**"
    echo
    echo '```'
    cat "${ARTIFACTS_DIR}/summary.txt"
    echo '```'
  } >> "${GITHUB_STEP_SUMMARY}"
fi

[[ ${fail} -eq 0 ]] || exit 1
