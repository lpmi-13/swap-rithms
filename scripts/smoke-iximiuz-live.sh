#!/usr/bin/env bash
# Run live smoke checks against a running iximiuz Labs playground session.
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/smoke-iximiuz-live.sh <playground-session-id>

Environment:
  MACHINE=lab-01              Target machine name.
  LAB_USER=laborant           SSH user.

This script assumes the playground is already running and initialized. Start one
with labctl after publishing/updating the manifest, for example:

  labctl playground start swap-rithms-325942ce --file playground/iximiuz/manifest.yaml --quiet --safety-disclaimer-consent
EOF
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

playground_id="${1:-}"
[ -n "$playground_id" ] || {
  usage >&2
  exit 1
}

machine="${MACHINE:-lab-01}"
user="${LAB_USER:-laborant}"
remote_script="/tmp/swap-rithms-iximiuz-live-smoke.sh"
local_script="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/iximiuz-live-smoke-remote.sh"

command -v labctl >/dev/null 2>&1 || {
  echo "labctl is required" >&2
  exit 1
}

labctl cp \
  --machine "$machine" \
  --user "$user" \
  "$local_script" \
  "${playground_id}:${remote_script}"

labctl ssh \
  --machine "$machine" \
  --user "$user" \
  "$playground_id" \
  -- env \
    SWAP_RITHMS_READY_TIMEOUT_SECONDS="${SWAP_RITHMS_READY_TIMEOUT_SECONDS:-120}" \
    bash "$remote_script"
