#!/usr/bin/env bash
# Manual batch e2e: applies all ChainInstance samples in batches and
# classifies the outcome of each (sync started / pod started / OOM / crash /
# image-pull / pending / no-status).
#
# Env:
#   BATCH_SIZE      (default 5)
#   READY_TIMEOUT   (default 600 sec per batch)
#   STORAGE_CLASS   (default "standard")
#   STORAGE_SIZE    (default 5Gi)
#   NAMESPACE       (default chainplane-batch)
#   CHAINS_FILTER   (regex on filename, empty = all)
#   ARTIFACTS_DIR   (default ./artifacts)
#   SAMPLES_DIR     (default config/samples)
#   POD_READY_GRACE (default 60 — sec a pod must stay Ready without crashes
#                    to count as PASS_STARTED)

set -uo pipefail

BATCH_SIZE="${BATCH_SIZE:-5}"
READY_TIMEOUT="${READY_TIMEOUT:-600}"
STORAGE_CLASS="${STORAGE_CLASS:-standard}"
STORAGE_SIZE="${STORAGE_SIZE:-1Gi}"
NAMESPACE="${NAMESPACE:-chainplane-batch}"
CHAINS_FILTER="${CHAINS_FILTER:-}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-artifacts}"
SAMPLES_DIR="${SAMPLES_DIR:-config/samples}"
POD_READY_GRACE="${POD_READY_GRACE:-60}"

mkdir -p "${ARTIFACTS_DIR}"

for bin in kubectl yq jq; do
  command -v "${bin}" >/dev/null 2>&1 || { echo "ERROR: ${bin} not in PATH"; exit 2; }
done

kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

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

echo "Discovered ${TOTAL} sample(s); batch=${BATCH_SIZE}; timeout=${READY_TIMEOUT}s; ready-grace=${POD_READY_GRACE}s"

declare -a RESULTS_NAME RESULTS_STATUS RESULTS_REASON

# Status taxonomy:
#   PASS_SYNC      .status.height >= 1
#   PASS_SOFT      pod Ready + .status.phase set (controller saw the node)
#   PASS_STARTED   pod Ready >= POD_READY_GRACE without container restarts
#   FAIL_OOM       any container OOMKilled
#   FAIL_CRASH     CrashLoopBackOff / RunContainerError / non-OOM termination
#   FAIL_IMAGE     ImagePullBackOff / ErrImagePull
#   FAIL_PENDING   PVC unbound / unschedulable / pod stuck Pending
#   FAIL_APPLY     kubectl apply rejected the manifest (CRD/admission)
#   FAIL_NO_STATUS pod Ready but controller never wrote .status

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

# Echoes one of: PASS_SYNC|PASS_SOFT|PASS_STARTED|FAIL_OOM|FAIL_CRASH|
# FAIL_IMAGE|FAIL_PENDING|FAIL_NO_STATUS|PROGRESS plus a reason string,
# separated by tab. PROGRESS = not done yet, keep waiting.
classify() {
  local nm="$1" first_ready_ts="$2"
  local height phase pod_json
  height="$(kubectl -n "${NAMESPACE}" get chaininstance "${nm}" -o jsonpath='{.status.height}' 2>/dev/null || true)"
  phase="$(kubectl -n "${NAMESPACE}" get chaininstance "${nm}" -o jsonpath='{.status.phase}' 2>/dev/null || true)"

  if [[ -n "${height}" && "${height}" =~ ^[1-9][0-9]*$ ]]; then
    printf 'PASS_SYNC\theight=%s phase=%s\n' "${height}" "${phase:-?}"
    return
  fi

  pod_json="$(kubectl -n "${NAMESPACE}" get pods -l "app=${nm}" -o json 2>/dev/null || echo '{"items":[]}')"
  local n_pods
  n_pods=$(jq '.items | length' <<<"${pod_json}")

  if [[ "${n_pods}" -eq 0 ]]; then
    printf 'PROGRESS\tno-pod-yet phase=%s\n' "${phase:-?}"
    return
  fi

  # Look at all containers across all pods for fault states.
  local oom crash image waiting_reason terminated_reason restart_count ready phase_pod
  oom=$(jq -r '[.items[].status.containerStatuses[]?.lastState.terminated // empty | select(.reason=="OOMKilled")] | length' <<<"${pod_json}")
  waiting_reason=$(jq -r '[.items[].status.containerStatuses[]?.state.waiting.reason // empty] | join(",")' <<<"${pod_json}")
  terminated_reason=$(jq -r '[.items[].status.containerStatuses[]?.lastState.terminated.reason // empty] | join(",")' <<<"${pod_json}")
  restart_count=$(jq -r '[.items[].status.containerStatuses[]?.restartCount // 0] | add // 0' <<<"${pod_json}")
  ready=$(jq -r '[.items[].status.conditions[]? | select(.type=="Ready") | .status] | join(",")' <<<"${pod_json}")
  phase_pod=$(jq -r '[.items[].status.phase] | join(",")' <<<"${pod_json}")

  if [[ "${oom}" -gt 0 ]]; then
    printf 'FAIL_OOM\trestarts=%s pod-phase=%s\n' "${restart_count}" "${phase_pod}"
    return
  fi
  if [[ "${waiting_reason}" =~ ImagePullBackOff|ErrImagePull|InvalidImageName ]]; then
    printf 'FAIL_IMAGE\twaiting=%s\n' "${waiting_reason}"
    return
  fi
  if [[ "${waiting_reason}" =~ CrashLoopBackOff|RunContainerError|CreateContainerConfigError ]]; then
    printf 'FAIL_CRASH\twaiting=%s last=%s restarts=%s\n' "${waiting_reason}" "${terminated_reason}" "${restart_count}"
    return
  fi
  if [[ "${phase_pod}" == "Pending" || "${phase_pod}" =~ Pending ]]; then
    printf 'PROGRESS\tpod-pending\n'
    return
  fi

  # Pod Ready?
  if [[ "${ready}" =~ True ]]; then
    if [[ "${first_ready_ts}" == "0" ]]; then
      # Caller will record the first-ready timestamp for grace tracking.
      printf 'JUST_READY\tpod-ready phase=%s\n' "${phase:-?}"
      return
    fi
    local now elapsed
    now=$(date +%s)
    elapsed=$(( now - first_ready_ts ))
    if [[ -n "${phase}" && "${phase}" != "Pending" ]]; then
      printf 'PASS_SOFT\tphase=%s ready-for=%ss\n' "${phase}" "${elapsed}"
      return
    fi
    if [[ ${elapsed} -ge ${POD_READY_GRACE} && ${restart_count} -eq 0 ]]; then
      printf 'PASS_STARTED\tready-for=%ss restarts=0\n' "${elapsed}"
      return
    fi
    printf 'PROGRESS\tready-but-no-status ready-for=%ss\n' "${elapsed}"
    return
  fi

  printf 'PROGRESS\tnot-ready phase=%s waiting=%s\n' "${phase_pod}" "${waiting_reason}"
}

dump_artifacts() {
  local name="$1"
  local logf="${ARTIFACTS_DIR}/${name}.log"
  {
    echo "=== describe chaininstance/${name} ==="
    kubectl -n "${NAMESPACE}" describe chaininstance "${name}" 2>&1 || true
    echo
    echo "=== describe pods ==="
    kubectl -n "${NAMESPACE}" describe pods -l "app=${name}" 2>&1 || true
    echo
    echo "=== pod logs (current + previous) ==="
    for p in $(kubectl -n "${NAMESPACE}" get pods -l "app=${name}" -o name 2>/dev/null); do
      echo "--- ${p} current ---"
      kubectl -n "${NAMESPACE}" logs "${p}" --all-containers --tail=200 2>&1 || true
      echo "--- ${p} previous ---"
      kubectl -n "${NAMESPACE}" logs "${p}" --all-containers --previous --tail=100 2>&1 || true
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
  local -A apply_failed=()

  for f in "${files[@]}"; do
    local nm
    nm="$(yq eval '.metadata.name' "$f")"
    names+=("$nm")
    patch_sample "$f" "${tmpdir}/${nm}.yaml"
  done

  echo "--- applying batch: ${names[*]}"
  local apply_log="${tmpdir}/apply.log"
  if ! kubectl apply -f "${tmpdir}/" >"${apply_log}" 2>&1; then
    cat "${apply_log}"
    # Mark per-file apply failures based on stderr text.
    while IFS= read -r line; do
      for nm in "${names[@]}"; do
        if [[ "${line}" =~ \"${nm}\" || "${line}" =~ /${nm}\.yaml ]]; then
          apply_failed[$nm]="${line}"
        fi
      done
    done < "${apply_log}"
  else
    cat "${apply_log}"
  fi

  local deadline=$(( $(date +%s) + READY_TIMEOUT ))
  local -A done_status=()
  local -A done_reason=()
  local -A first_ready=()

  # Pre-fail the ones that didn't even apply.
  for nm in "${names[@]}"; do
    if [[ -n "${apply_failed[$nm]:-}" ]]; then
      done_status[$nm]="FAIL_APPLY"
      done_reason[$nm]="${apply_failed[$nm]}"
      echo "    FAIL_APPLY ${nm}"
    fi
  done

  while [[ $(date +%s) -lt ${deadline} ]]; do
    local all_done=1
    for nm in "${names[@]}"; do
      [[ -n "${done_status[$nm]:-}" ]] && continue
      local out status reason
      out="$(classify "${nm}" "${first_ready[$nm]:-0}")"
      status="${out%%$'\t'*}"
      reason="${out#*$'\t'}"
      case "${status}" in
        JUST_READY)
          first_ready[$nm]=$(date +%s)
          all_done=0
          ;;
        PROGRESS)
          all_done=0
          ;;
        *)
          done_status[$nm]="${status}"
          done_reason[$nm]="${reason}"
          echo "    ${status} ${nm} (${reason})"
          ;;
      esac
    done
    [[ ${all_done} -eq 1 ]] && break
    sleep 10
  done

  for nm in "${names[@]}"; do
    if [[ -z "${done_status[$nm]:-}" ]]; then
      # Final classify pass — turn JUST_READY/PROGRESS into best terminal state.
      local out status reason
      out="$(classify "${nm}" "${first_ready[$nm]:-0}")"
      status="${out%%$'\t'*}"
      reason="${out#*$'\t'}"
      case "${status}" in
        PASS_*|FAIL_*) done_status[$nm]="${status}"; done_reason[$nm]="${reason}" ;;
        *)             done_status[$nm]="FAIL_NO_STATUS"; done_reason[$nm]="timeout: ${reason}" ;;
      esac
      echo "    ${done_status[$nm]} ${nm} (${done_reason[$nm]})"
    fi
    RESULTS_NAME+=("${nm}")
    RESULTS_STATUS+=("${done_status[$nm]}")
    RESULTS_REASON+=("${done_reason[$nm]}")
    [[ "${done_status[$nm]}" =~ ^FAIL_ ]] && dump_artifacts "${nm}"
  done

  kubectl delete -f "${tmpdir}/" --wait=true --timeout=60s >/dev/null 2>&1 || true
  # PVCs created by StatefulSet aren't auto-deleted; nuke everything in the
  # batch namespace so the next batch starts clean.
  kubectl -n "${NAMESPACE}" delete pvc --all --wait=false >/dev/null 2>&1 || true
  rm -rf "${tmpdir}"
}

i=0
while [[ ${i} -lt ${TOTAL} ]]; do
  end=$(( i + BATCH_SIZE ))
  [[ ${end} -gt ${TOTAL} ]] && end=${TOTAL}
  echo
  echo "=== batch $((i / BATCH_SIZE + 1)) [$((i+1))-${end}/${TOTAL}] ==="
  df -h / 2>&1 | tail -1 | awk '{printf "    disk: %s free, %s used (%s)\n", $4, $3, $5}'
  run_batch "${ALL_SAMPLES[@]:i:$((end - i))}"
  i=${end}
done

echo
echo "=== SUMMARY ==="
{
  printf "| %-40s | %-15s | %s\n" "chain" "status" "reason"
  printf "|%s|%s|%s\n" "$(printf '%.s-' {1..42})" "$(printf '%.s-' {1..17})" "$(printf '%.s-' {1..40})"
  for n in "${!RESULTS_NAME[@]}"; do
    printf "| %-40s | %-15s | %s\n" "${RESULTS_NAME[$n]}" "${RESULTS_STATUS[$n]}" "${RESULTS_REASON[$n]}"
  done
} > "${ARTIFACTS_DIR}/summary.txt"
cat "${ARTIFACTS_DIR}/summary.txt"

declare -A counts
for n in "${!RESULTS_NAME[@]}"; do
  s="${RESULTS_STATUS[$n]}"
  counts[$s]=$(( ${counts[$s]:-0} + 1 ))
done

pass=0; fail=0
for s in "${!counts[@]}"; do
  [[ "$s" =~ ^PASS_ ]] && pass=$(( pass + counts[$s] ))
  [[ "$s" =~ ^FAIL_ ]] && fail=$(( fail + counts[$s] ))
done

echo
echo "totals: pass=${pass} fail=${fail} total=${#RESULTS_NAME[@]}"
echo "breakdown:"
for s in PASS_SYNC PASS_SOFT PASS_STARTED FAIL_OOM FAIL_CRASH FAIL_IMAGE FAIL_PENDING FAIL_APPLY FAIL_NO_STATUS; do
  echo "  ${s}=${counts[$s]:-0}"
done

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "## Manual batch e2e summary"
    echo
    echo "**pass=${pass}** **fail=${fail}** total=${#RESULTS_NAME[@]}"
    echo
    echo "| status | count |"
    echo "|---|---|"
    for s in PASS_SYNC PASS_SOFT PASS_STARTED FAIL_OOM FAIL_CRASH FAIL_IMAGE FAIL_PENDING FAIL_APPLY FAIL_NO_STATUS; do
      echo "| ${s} | ${counts[$s]:-0} |"
    done
    echo
    echo '<details><summary>Per-chain results</summary>'
    echo
    echo '```'
    cat "${ARTIFACTS_DIR}/summary.txt"
    echo '```'
    echo '</details>'
  } >> "${GITHUB_STEP_SUMMARY}"
fi

[[ ${fail} -eq 0 ]] || exit 1
