#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${PANEL_INTEGRATION_ENV_FILE:-$ROOT_DIR/.env.integration.local}"
EXAMPLE_ENV_FILE="$ROOT_DIR/.env.integration.example"

load_env_file() {
  local file="$1"
  if [[ -f "$file" ]]; then
    set -a
    # shellcheck disable=SC1090
    source "$file"
    set +a
  fi
}

usage() {
  cat <<'USAGE'
Usage: scripts/panel-integration.sh <command>

Commands:
  help        Show this help.
  summary     Print redacted live integration configuration.
  run         Start rw-node-go, enable Panel node, wait for connection, run smoke, disable node, then stop rw-node-go.
  start       Start rw-node-go in the background.
  stop        Stop the background rw-node-go started by this script.
  status      Show process and local endpoint status.
  smoke       Run only the live Panel smoke test.
  node        Query Panel for the current node status summary.
  enable      Enable Panel node and wait until Panel reports it connected.
  disable     Disable Panel node.

Configuration:
  Copy .env.integration.example to .env.integration.local and fill:
  PANEL_BASE_URL, PANEL_API_KEY, PANEL_NODE_ID, SECRET_KEY.

Optional:
  PANEL_SMOKE_PATH, PANEL_NODE_ID, NODE_PORT, INTERNAL_REST_PORT, XRAY_ASSET_DIR,
  PANEL_INTEGRATION_LOG_DIR, PANEL_INTEGRATION_BIN_DIR.
USAGE
}

json_escape() {
  local value="${1:-}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/}"
  printf '%s' "$value"
}

redact() {
  local value="${1:-}"
  local length=${#value}
  if [[ -z "$value" ]]; then
    printf ''
  elif (( length <= 8 )); then
    printf '[REDACTED]'
  else
    printf '%s...%s' "${value:0:4}" "${value: -4}"
  fi
}

log_event() {
  local level="$1"
  local event="$2"
  local message="$3"
  printf '{"ts":"%s","level":"%s","event":"%s","message":"%s"}\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    "$(json_escape "$level")" \
    "$(json_escape "$event")" \
    "$(json_escape "$message")"
}

require_env() {
  local missing=()
  for name in "$@"; do
    if [[ -z "${!name:-}" ]]; then
      missing+=("$name")
    fi
  done
  if (( ${#missing[@]} > 0 )); then
    log_event "error" "missing_env" "Missing required env: ${missing[*]}. Copy .env.integration.example to .env.integration.local and fill it."
    exit 2
  fi
}

require_full_node_uuid() {
  require_env PANEL_NODE_ID
  if ! [[ "$PANEL_NODE_ID" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
    log_event "error" "panel_node_id_not_uuid" "PANEL_NODE_ID must be a full node UUID for run/enable/disable because these commands modify real Panel node state. Use node for read-only fuzzy lookup."
    exit 2
  fi
}

init_runtime() {
  load_env_file "$ENV_FILE"

  NODE_PORT="${NODE_PORT:-2222}"
  INTERNAL_REST_PORT="${INTERNAL_REST_PORT:-61001}"
  PANEL_SMOKE_PATH="${PANEL_SMOKE_PATH:-/api/system/metadata}"
  PANEL_INTEGRATION_LOG_DIR="${PANEL_INTEGRATION_LOG_DIR:-logs/panel-integration}"
  PANEL_INTEGRATION_BIN_DIR="${PANEL_INTEGRATION_BIN_DIR:-runtime/bin}"
  XRAY_ASSET_DIR="${XRAY_ASSET_DIR:-runtime/xray}"
  LOG_LEVEL="${LOG_LEVEL:-info}"
  RW_PANEL_INTEGRATION_NODE_MARKER="rw-node-go-panel-integration"

  case "$PANEL_INTEGRATION_LOG_DIR" in
    /*|[A-Za-z]:/*|[A-Za-z]:\\*) LOG_DIR="$PANEL_INTEGRATION_LOG_DIR" ;;
    *) LOG_DIR="$ROOT_DIR/$PANEL_INTEGRATION_LOG_DIR" ;;
  esac
  PID_FILE="$LOG_DIR/rw-node-go.pid.json"
  mkdir -p "$LOG_DIR"

  case "$PANEL_INTEGRATION_BIN_DIR" in
    /*|[A-Za-z]:/*|[A-Za-z]:\\*) ;;
    *) PANEL_INTEGRATION_BIN_DIR="$ROOT_DIR/$PANEL_INTEGRATION_BIN_DIR" ;;
  esac
  mkdir -p "$PANEL_INTEGRATION_BIN_DIR"

  case "$XRAY_ASSET_DIR" in
    /*|[A-Za-z]:/*|[A-Za-z]:\\*) ;;
    *) XRAY_ASSET_DIR="$ROOT_DIR/$XRAY_ASSET_DIR" ;;
  esac
  mkdir -p "$XRAY_ASSET_DIR"
  XRAY_ASSET_ENV="$XRAY_ASSET_DIR"
  if [[ -n "${MSYSTEM:-}" ]] && command -v cygpath >/dev/null 2>&1; then
    XRAY_ASSET_ENV="$(cygpath -w "$XRAY_ASSET_DIR")"
  fi
}

print_summary() {
  init_runtime
  cat <<SUMMARY
{
  "panel_base_url": "$(json_escape "${PANEL_BASE_URL:-}")",
  "panel_api_key": "$(json_escape "$(redact "${PANEL_API_KEY:-}")")",
  "panel_node_id": "$(json_escape "${PANEL_NODE_ID:-}")",
  "panel_smoke_path": "$(json_escape "${PANEL_SMOKE_PATH:-}")",
  "secret_key": "$(json_escape "$(redact "${SECRET_KEY:-}")")",
  "node_port": "$(json_escape "${NODE_PORT:-}")",
  "internal_rest_port": "$(json_escape "${INTERNAL_REST_PORT:-}")",
  "xray_asset_dir": "$(json_escape "$XRAY_ASSET_DIR")",
  "xray_asset_env": "$(json_escape "$XRAY_ASSET_ENV")",
  "xray_geoip_dat": "$(json_escape "$(asset_status geoip.dat)")",
  "xray_geosite_dat": "$(json_escape "$(asset_status geosite.dat)")",
  "bin_dir": "$(json_escape "$PANEL_INTEGRATION_BIN_DIR")",
  "log_dir": "$(json_escape "$LOG_DIR")",
  "env_file": "$(json_escape "$ENV_FILE")"
}
SUMMARY
}

asset_status() {
  local name="$1"
  if [[ -s "$XRAY_ASSET_DIR/$name" ]]; then
    printf 'present'
  else
    printf 'missing'
  fi
}

go_cmd() {
  if command -v mise >/dev/null 2>&1; then
    mise exec -- go "$@"
  else
    go "$@"
  fi
}

run_panel_harness() {
  local command="$1"
  shift || true
  (
    cd "$ROOT_DIR"
    RW_PANEL_INTEGRATION_SCRIPT=1 \
    PANEL_BASE_URL="${PANEL_BASE_URL:-}" \
    PANEL_API_KEY="${PANEL_API_KEY:-}" \
    PANEL_NODE_ID="${PANEL_NODE_ID:-}" \
    PANEL_SMOKE_PATH="${PANEL_SMOKE_PATH:-}" \
    go_cmd run ./cmd/panel-integration "$command" "$@"
  )
}

build_node_binary() {
  init_runtime
  local goexe binary build_output
  goexe="$(go_cmd env GOEXE 2>/dev/null || true)"
  binary="$PANEL_INTEGRATION_BIN_DIR/rw-node-go$goexe"
  build_output="$binary"
  if [[ -n "${MSYSTEM:-}" ]] && command -v cygpath >/dev/null 2>&1; then
    build_output="$(cygpath -w "$binary")"
  fi
  log_event "info" "node_build" "building rw-node-go binary at $binary" >&2
  (
    cd "$ROOT_DIR"
    go_cmd run ./cmd/rw-build -repo-root "$ROOT_DIR" -o "$build_output"
  ) >&2
  printf '%s' "$binary"
}

pid_file_is_legacy() {
  [[ -f "$PID_FILE" ]] || return 1
  local first
  first="$(head -c 1 "$PID_FILE" 2>/dev/null || true)"
  [[ "$first" != "{" ]]
}

pid_value() {
  [[ -f "$PID_FILE" ]] || return 1
  if pid_file_is_legacy; then
    return 1
  fi
  sed -n 's/.*"pid"[[:space:]]*:[[:space:]]*\([0-9][0-9]*\).*/\1/p' "$PID_FILE" | head -n 1
}

pid_binary_value() {
  [[ -f "$PID_FILE" ]] || return 1
  if pid_file_is_legacy; then
    return 1
  fi
  sed -n 's/.*"binary"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$PID_FILE" | head -n 1
}

write_pid_file() {
  local pid="$1"
  local binary="$2"
  local started_at="$3"
  cat >"$PID_FILE" <<PIDJSON
{"pid":$pid,"binary":"$(json_escape "$binary")","started_at":"$(json_escape "$started_at")","marker":"$(json_escape "$RW_PANEL_INTEGRATION_NODE_MARKER")"}
PIDJSON
}

process_identity_matches() {
  local pid="$1"
  local expected_binary="$2"
  [[ -n "$pid" && -n "$expected_binary" ]] || return 1

  if [[ -d "/proc/$pid" ]]; then
    local cmdline environ exe
    cmdline="$(tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null || true)"
    environ="$(tr '\0' '\n' <"/proc/$pid/environ" 2>/dev/null || true)"
    exe="$(readlink "/proc/$pid/exe" 2>/dev/null || true)"
    if [[ "$environ" == *"RW_PANEL_INTEGRATION_NODE=1"* && "$environ" == *"RW_PANEL_INTEGRATION_NODE_MARKER=$RW_PANEL_INTEGRATION_NODE_MARKER"* ]]; then
      return 0
    fi
    if [[ -n "$cmdline" && "$cmdline" == *"$expected_binary"* ]]; then
      return 0
    fi
    if [[ -n "$exe" && "$exe" == "$expected_binary" ]]; then
      return 0
    fi
    return 1
  fi

  if command -v powershell.exe >/dev/null 2>&1; then
    local escaped expected proc_path proc_cmd
    escaped="${pid//\'/\'\'}"
    proc_path="$(powershell.exe -NoProfile -Command "try { (Get-CimInstance Win32_Process -Filter \"ProcessId=$escaped\").ExecutablePath } catch { '' }" 2>/dev/null | tr -d '\r' | head -n 1 || true)"
    proc_cmd="$(powershell.exe -NoProfile -Command "try { (Get-CimInstance Win32_Process -Filter \"ProcessId=$escaped\").CommandLine } catch { '' }" 2>/dev/null | tr -d '\r' | head -n 1 || true)"
    expected="$expected_binary"
    if [[ -n "${MSYSTEM:-}" ]] && command -v cygpath >/dev/null 2>&1; then
      expected="$(cygpath -w "$expected_binary")"
    fi
    if [[ -n "$proc_path" && "$proc_path" == "$expected" ]]; then
      return 0
    fi
    if [[ -n "$proc_cmd" && "$proc_cmd" == *"$expected"* ]]; then
      return 0
    fi
  fi

  return 1
}

is_running() {
  [[ -f "$PID_FILE" ]] || return 1
  if pid_file_is_legacy; then
    log_event "warn" "stale_pid_file" "legacy pid file found at $PID_FILE; refusing to trust it. Remove it after confirming no harness node is running."
    return 1
  fi
  local pid binary
  pid="$(pid_value || true)"
  binary="$(pid_binary_value || true)"
  [[ -n "$pid" ]] || return 1
  kill -0 "$pid" >/dev/null 2>&1 || return 1
  process_identity_matches "$pid" "$binary"
}

start_node() {
  init_runtime
  require_env SECRET_KEY
  if is_running; then
    log_event "info" "node_start" "rw-node-go is already running with pid $(pid_value)"
    return
  fi

  local stamp log_file pid binary
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  log_file="$LOG_DIR/rw-node-go-$stamp.log"
  binary="$(build_node_binary)"

  log_event "info" "node_start" "starting rw-node-go; log=$log_file"
  if [[ "$(asset_status geoip.dat)" != "present" || "$(asset_status geosite.dat)" != "present" ]]; then
    log_event "warn" "xray_assets_missing" "put geoip.dat and geosite.dat in $XRAY_ASSET_DIR"
  fi
  (
    cd "$ROOT_DIR"
    exec env \
      "XRAY_LOCATION_ASSET=$XRAY_ASSET_ENV" \
      "xray.location.asset=$XRAY_ASSET_ENV" \
      "NODE_PORT=$NODE_PORT" \
      "INTERNAL_REST_PORT=$INTERNAL_REST_PORT" \
      "SECRET_KEY=$SECRET_KEY" \
      "LOG_LEVEL=$LOG_LEVEL" \
      "RW_PANEL_INTEGRATION_NODE=1" \
      "RW_PANEL_INTEGRATION_NODE_MARKER=$RW_PANEL_INTEGRATION_NODE_MARKER" \
      "$binary"
  ) >"$log_file" 2>&1 &
  pid=$!
  write_pid_file "$pid" "$binary" "$stamp"

  sleep 2
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    log_event "error" "node_start_failed" "rw-node-go exited early; inspect $log_file"
    rm -f "$PID_FILE"
    exit 1
  fi
  if ! process_identity_matches "$pid" "$binary"; then
    log_event "error" "node_start_failed" "rw-node-go pid=$pid could not be verified as a harness-owned process; inspect $log_file"
    rm -f "$PID_FILE"
    exit 1
  fi
  log_event "info" "node_started" "pid=$pid"
}

stop_node() {
  init_runtime
  if ! [[ -f "$PID_FILE" ]]; then
    log_event "info" "node_stop" "no pid file found"
    return
  fi

  if pid_file_is_legacy; then
    log_event "error" "stale_pid_file" "legacy pid file found at $PID_FILE; refusing to kill an unverified process. Remove it after confirming no harness node is running."
    return 1
  fi

  local pid binary
  pid="$(pid_value || true)"
  binary="$(pid_binary_value || true)"
  if [[ -z "$pid" ]]; then
    rm -f "$PID_FILE"
    log_event "warn" "node_stop" "empty pid file removed"
    return
  fi

  if kill -0 "$pid" >/dev/null 2>&1; then
    if ! process_identity_matches "$pid" "$binary"; then
      log_event "error" "stale_pid_file" "pid=$pid from $PID_FILE is running but does not match this harness; refusing to send SIGTERM"
      return 1
    fi
    log_event "info" "node_stop" "stopping pid=$pid"
    kill "$pid" >/dev/null 2>&1 || true
    for _ in 1 2 3 4 5; do
      if ! kill -0 "$pid" >/dev/null 2>&1; then
        break
      fi
      sleep 1
    done
    if kill -0 "$pid" >/dev/null 2>&1; then
      log_event "warn" "node_stop" "pid=$pid did not exit after SIGTERM"
    fi
  else
    log_event "info" "node_stop" "pid=$pid is not running"
  fi
  rm -f "$PID_FILE"
}

status_node() {
  init_runtime
  print_summary
  if is_running; then
    log_event "info" "node_status" "running pid=$(pid_value)"
  else
    log_event "info" "node_status" "not running"
  fi
  if command -v curl >/dev/null 2>&1; then
    local internal_url="http://127.0.0.1:$INTERNAL_REST_PORT/internal/get-config"
    local code
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 2 "$internal_url" 2>/dev/null || true)"
    log_event "info" "internal_get_config" "url=$internal_url http_code=${code:-000}"
  else
    log_event "warn" "curl_missing" "curl is not available; skipped internal endpoint probe"
  fi
}

panel_node_status() {
  init_runtime
  require_env PANEL_BASE_URL PANEL_API_KEY PANEL_NODE_ID
  log_event "info" "panel_node_status" "running live Panel node status lookup"
  run_panel_harness node
}

run_smoke() {
  init_runtime
  require_env PANEL_BASE_URL PANEL_API_KEY
  log_event "info" "panel_smoke" "running live Panel smoke test"
  run_panel_harness smoke
}

enable_panel_node() {
  init_runtime
  require_env PANEL_BASE_URL PANEL_API_KEY
  require_full_node_uuid
  log_event "info" "panel_node_enable" "enabling Panel node and waiting for connected status"
  if [[ "${1:-}" == "standalone" ]]; then
    log_event "warn" "panel_node_enable_cleanup_notice" "standalone enable leaves the Panel test node enabled; run bash scripts/panel-integration.sh disable when finished"
  fi
  run_panel_harness enable
}

disable_panel_node() {
  init_runtime
  require_env PANEL_BASE_URL PANEL_API_KEY
  require_full_node_uuid
  log_event "info" "panel_node_disable" "disabling Panel node"
  run_panel_harness disable
}

cleanup_run() {
  local exit_code=$?
  set +e
  if [[ "${PANEL_NODE_ENABLED_BY_RUN:-0}" == "1" ]]; then
    disable_panel_node
    local disable_status=$?
    if (( disable_status != 0 )); then
      log_event "error" "panel_node_disable_cleanup_failed" "cleanup disable failed with exit_code=$disable_status"
      if (( exit_code == 0 )); then
        exit_code=$disable_status
      fi
    else
      PANEL_NODE_ENABLED_BY_RUN=0
    fi
  fi
  stop_node
  local stop_status=$?
  if (( stop_status != 0 )); then
    log_event "error" "node_stop_cleanup_failed" "cleanup stop failed with exit_code=$stop_status"
    if (( exit_code == 0 )); then
      exit_code=$stop_status
    fi
  fi
  exit "$exit_code"
}

run_all() {
  init_runtime
  require_env PANEL_BASE_URL PANEL_API_KEY SECRET_KEY
  require_full_node_uuid
  print_summary
  PANEL_NODE_ENABLED_BY_RUN=0
  start_node
  trap cleanup_run EXIT
  status_node
  enable_panel_node
  PANEL_NODE_ENABLED_BY_RUN=1
  run_smoke
  disable_panel_node
  PANEL_NODE_ENABLED_BY_RUN=0
}

main() {
  if [[ -n "${MSYSTEM:-}" ]]; then
    export MSYS_NO_PATHCONV=1
    export MSYS2_ARG_CONV_EXCL='*'
  fi
  local command="${1:-help}"
  case "$command" in
    help|-h|--help) usage ;;
    summary) print_summary ;;
    run) run_all ;;
    start) start_node ;;
    stop) stop_node ;;
    status) status_node ;;
    smoke) run_smoke ;;
    node) panel_node_status ;;
    enable) enable_panel_node standalone ;;
    disable) disable_panel_node ;;
    *)
      usage
      log_event "error" "unknown_command" "$command"
      exit 2
      ;;
  esac
}

main "$@"
