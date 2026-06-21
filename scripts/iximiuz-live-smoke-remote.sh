#!/usr/bin/env bash
# Runs inside the iximiuz VM. Use scripts/smoke-iximiuz-live.sh from the local
# repo to copy and execute this script through labctl.
set -euo pipefail

root="${SWAP_RITHMS_ROOT:-/opt/swap-rithms}"
base_url="${SWAP_RITHMS_URL:-http://127.0.0.1:8080}"
timeout_seconds="${SWAP_RITHMS_READY_TIMEOUT_SECONDS:-120}"
state_file="/tmp/swap-rithms-state.json"
recent_file="/tmp/swap-rithms-recent.json"

log() {
  printf '\n== %s ==\n' "$*"
}

need_executable() {
  local path="$1"
  [ -x "$path" ] || {
    echo "missing executable: $path" >&2
    exit 1
  }
}

need_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || {
    echo "missing expected command: $cmd" >&2
    exit 1
  }
}

wait_ready() {
  local deadline=$((SECONDS + timeout_seconds))

  while [ "$SECONDS" -lt "$deadline" ]; do
    if curl -fsS "${base_url}/api/state" -o "$state_file" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "swap-rithms did not become ready within ${timeout_seconds}s" >&2
  systemctl status swap-rithms.service --no-pager >&2 || true
  journalctl -u swap-rithms.service --no-pager -n 80 >&2 || true
  return 1
}

log "readiness"
wait_ready

log "installed files"
need_executable "${root}/swap-rithms"
need_executable /usr/local/bin/swap-rithms
need_cmd curl
need_cmd jq
need_cmd node
need_cmd python3

log "service"
systemctl is-active --quiet swap-rithms.service

log "runtime availability"
jq -e '.languages[] | select(.name == "go" and .available == true)' "$state_file" >/dev/null
jq -e '.languages[] | select(.name == "python" and .available == true)' "$state_file" >/dev/null
jq -e '.languages[] | select(.name == "typescript" and .available == true)' "$state_file" >/dev/null

log "profile lookup"
curl -fsS "${base_url}/profiles/recent?window=5m&ids=false" -o "$recent_file"
jq -e '.count >= 0 and .elapsedMicros >= 0' "$recent_file" >/dev/null

log "typescript switch"
curl -fsS \
  -H 'content-type: application/json' \
  -d '{"language":"typescript","name":"binary_search"}' \
  "${base_url}/api/algorithm" >/dev/null
curl -fsS "${base_url}/profiles/recent?window=10m&ids=false" -o "$recent_file"
jq -e '.language == "typescript" and .algorithm == "binary_search"' "$recent_file" >/dev/null

log "python switch"
curl -fsS \
  -H 'content-type: application/json' \
  -d '{"language":"python","name":"bucketed_index"}' \
  "${base_url}/api/algorithm" >/dev/null
curl -fsS "${base_url}/profiles/recent?window=10m&ids=false" -o "$recent_file"
jq -e '.language == "python" and .algorithm == "bucketed_index"' "$recent_file" >/dev/null

log "metrics"
curl -fsS "${base_url}/metrics" | grep -q '^swap_rithms_requests_total'

echo
echo "iximiuz live smoke passed"
