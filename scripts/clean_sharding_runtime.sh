#!/usr/bin/env bash
set -euo pipefail

# Clean runtime state for the sharding release packages on the current machine.
# It removes ledger/runtime data and logs, but keeps binaries, configs and certs.
#
# Default target:
#   ${SOURCE_DIR}/build/release/*/chainmaker-*/data
#   ${SOURCE_DIR}/build/release/*/chainmaker-*/log
#
# Usage:
#   bash scripts/clean_sharding_runtime.sh
#   DRY_RUN=true bash scripts/clean_sharding_runtime.sh
#   FORCE=true bash scripts/clean_sharding_runtime.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -z "${SOURCE_DIR:-}" ]]; then
  if [[ -d "${SCRIPT_DIR}/../build/release" ]]; then
    SOURCE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
  elif [[ -d "${SCRIPT_DIR}/../chainmaker-go-v2.4.0_alpha/build/release" ]]; then
    SOURCE_DIR="$(cd "${SCRIPT_DIR}/../chainmaker-go-v2.4.0_alpha" && pwd)"
  else
    SOURCE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
  fi
fi
RELEASE_DIR="${RELEASE_DIR:-${SOURCE_DIR}/build/release}"
DRY_RUN="${DRY_RUN:-false}"
FORCE="${FORCE:-false}"

die() {
  echo "ERROR: $*" >&2
  exit 1
}

is_true() {
  case "${1:-}" in
    true|TRUE|1|yes|YES) return 0 ;;
    *) return 1 ;;
  esac
}

remove_path() {
  local path="$1"
  if rm -rf -- "${path}" 2>/tmp/clean_sharding_runtime_rm.err; then
    rm -f /tmp/clean_sharding_runtime_rm.err
    return 0
  fi
  echo "  normal rm failed, retry with sudo: ${path}" >&2
  if command -v sudo >/dev/null 2>&1; then
    sudo rm -rf -- "${path}"
    rm -f /tmp/clean_sharding_runtime_rm.err
    return 0
  fi
  cat /tmp/clean_sharding_runtime_rm.err >&2 || true
  rm -f /tmp/clean_sharding_runtime_rm.err
  return 1
}

[[ -d "${RELEASE_DIR}" ]] || die "release directory not found: ${RELEASE_DIR}"

running_processes="$(
  ps -eo pid=,args= | awk '
    $2 == "chainmaker" ||
    $2 == "./chainmaker" ||
    $2 ~ /\/chainmaker$/ ||
    $2 ~ /\/chainmaker-gateway$/ {
      print
    }'
)"
if [[ -n "${running_processes}" ]] && ! is_true "${FORCE}"; then
  cat >&2 <<EOF
ERROR: chainmaker related processes are still running.
Stop nodes/gateway first, or rerun with FORCE=true if you are sure.

${running_processes}
EOF
  exit 1
fi

mapfile -t targets < <(
  find "${RELEASE_DIR}" -mindepth 3 -maxdepth 3 -type d \
    \( -path '*/chainmaker-*/data' -o -path '*/chainmaker-*/log' \) \
    | sort
)

if [[ "${#targets[@]}" -eq 0 ]]; then
  echo "No runtime data/log directories found under: ${RELEASE_DIR}"
  exit 0
fi

echo "===> Clean sharding runtime state"
echo "source_dir=${SOURCE_DIR}"
echo "release_dir=${RELEASE_DIR}"
echo "targets=${#targets[@]}"
echo "dry_run=${DRY_RUN}"

for path in "${targets[@]}"; do
  if is_true "${DRY_RUN}"; then
    echo "would remove: ${path}"
  else
    echo "remove: ${path}"
    remove_path "${path}"
  fi
done

if ! is_true "${DRY_RUN}"; then
  echo "done: runtime data/logs removed. Configs, certs, binaries and libs are kept."
fi
