#!/bin/bash

set -euo pipefail

SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
ARTIFACT_ROOT="${SCRIPT_DIR}/artifacts"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
ARTIFACT_DIR="${ARTIFACT_ROOT}/${TIMESTAMP}"
DEFAULT_SERVER_PORT=3868

RUNNER_MODE=${RUNNER_MODE:-docker}
MODE=${MODE:-diameter}
SERVER_HOST=${SERVER_HOST:-}
SERVER_PORT=${SERVER_PORT:-}
SERVICE_NAME=${SERVICE_NAME:-}
SERVICE_NAMESPACE=${SERVICE_NAMESPACE:-default}
MASTER_VM=${MASTER_VM:-master}
CLIENT_VM=${CLIENT_VM:-bastion}
ATTEMPTS=${ATTEMPTS:-5}
IDLE_SECONDS=${IDLE_SECONDS:-300}
IDLE_JITTER=${IDLE_JITTER:-15}
IDLE_MODE=${IDLE_MODE:-quiet}
DWR_INTERVAL=${DWR_INTERVAL:-30}
BETWEEN_ATTEMPTS=${BETWEEN_ATTEMPTS:-3}
WAIT_FOR_DPA=${WAIT_FOR_DPA:-yes}
WAIT_FOR_DWA=${WAIT_FOR_DWA:-no}
CLOSE_MODE=${CLOSE_MODE:-client}
REQUIRE_PEER_CLOSE=${REQUIRE_PEER_CLOSE:-yes}
FAIL_ON_ATTEMPT_ERROR=${FAIL_ON_ATTEMPT_ERROR:-yes}
ORIGIN_HOST=${ORIGIN_HOST:-client.localdomain}
ORIGIN_REALM=${ORIGIN_REALM:-localdomain}
DESTINATION_HOST=${DESTINATION_HOST:-}
DESTINATION_REALM=${DESTINATION_REALM:-}
HOST_IP_AVP=${HOST_IP_AVP:-}
PRODUCT_NAME=${PRODUCT_NAME:-loxilb-dpr-loop}
VENDOR_ID=${VENDOR_ID:-10415}
DISCONNECT_CAUSE=${DISCONNECT_CAUSE:-2}
LOCAL_IMAGE=${LOCAL_IMAGE:-local/loxilb-dpr-loop:latest}
REBUILD_IMAGE=${REBUILD_IMAGE:-no}
KUBECONFIG=${KUBECONFIG:-}

SUMMARY_PATH="${ARTIFACT_DIR}/summary.json"
EVENT_LOG_PATH="${ARTIFACT_DIR}/events.jsonl"
STDOUT_LOG_PATH="${ARTIFACT_DIR}/runner.log"
CONTAINER_SUMMARY_PATH="/artifacts/summary.json"
CONTAINER_EVENT_LOG_PATH="/artifacts/events.jsonl"

SERVER_HOST_SOURCE=${SERVER_HOST:+env}
SERVER_PORT_SOURCE=${SERVER_PORT:+env}
HOST_IP_AVP_SOURCE=${HOST_IP_AVP:+env}
DISCOVERED_SERVICE_NAME=
DISCOVERED_SERVICE_NAMESPACE=

mkdir -p "${ARTIFACT_DIR}"

log() {
  echo "[$(date +%H:%M:%S)] $*"
}

fail() {
  log "$*"
  exit 1
}

bool_arg() {
  local value=${1,,}
  local flag=$2
  if [[ "$value" == "yes" || "$value" == "true" || "$value" == "1" ]]; then
    echo "$flag"
  fi
}

ensure_cmd() {
  local cmd=$1
  command -v "$cmd" >/dev/null 2>&1 || fail "Required command not found: $cmd"
}

parse_service_discovery_json() {
  python3 - "$SERVICE_NAME" "$SERVICE_NAMESPACE" <<'PY'
import json
import sys

service_name = (sys.argv[1] or "").strip()
service_namespace = (sys.argv[2] or "default").strip() or "default"
preferred_names = [
    "multus-seagull-service",
    "multus-sctp-service",
    "multus-service",
    "sctp-lb1",
]

try:
    data = json.load(sys.stdin)
except json.JSONDecodeError:
    sys.exit(1)


def ingress_host(service):
    ingress = (((service.get("status") or {}).get("loadBalancer") or {}).get("ingress") or [])
    if not ingress:
        return ""
    first = ingress[0] or {}
    return (first.get("ip") or first.get("hostname") or "").strip()


def sctp_port(service):
    for port in (service.get("spec") or {}).get("ports") or []:
        if str(port.get("protocol") or "").upper() == "SCTP" and port.get("port"):
            return str(port["port"])
    return ""


candidates = []
for service in data.get("items") or []:
    metadata = service.get("metadata") or {}
    namespace = (metadata.get("namespace") or "default").strip() or "default"
    name = (metadata.get("name") or "").strip()
    host = ingress_host(service)
    port = sctp_port(service)
    if not host or not port:
        continue

    if service_name:
        if name == service_name and namespace == service_namespace:
            print("\t".join((namespace, name, host, port)))
            sys.exit(0)
        continue

    score = 0
    if name in preferred_names:
        score += 100 - preferred_names.index(name)
    if namespace == service_namespace:
        score += 10
    if "sctp" in name:
        score += 5
    if "seagull" in name:
        score += 3
    candidates.append((score, namespace, name, host, port))

if service_name:
    sys.exit(1)

if not candidates:
    sys.exit(1)

candidates.sort(key=lambda item: (-item[0], item[1], item[2]))
best = candidates[0]
print("\t".join(best[1:]))
PY
}

discover_service_local() {
  command -v kubectl >/dev/null 2>&1 || return 0
  command -v python3 >/dev/null 2>&1 || return 0
  local kube_opt=()
  if [[ -n "$KUBECONFIG" ]]; then
    kube_opt+=(--kubeconfig "$KUBECONFIG")
  fi
  kubectl "${kube_opt[@]}" get svc -A -o json 2>/dev/null | parse_service_discovery_json || true
}

discover_service_vagrant() {
  command -v vagrant >/dev/null 2>&1 || return 0
  command -v python3 >/dev/null 2>&1 || return 0
  vagrant ssh "$MASTER_VM" -c "sudo kubectl get svc -A -o json" 2>/dev/null | tr -d '\r' | parse_service_discovery_json || true
}

discover_server_host_from_extip() {
  local file_path host
  for file_path in "$SCRIPT_DIR/extIP" "$SCRIPT_DIR/../k3s-sctpmh-seagull/extIP"; do
    if [[ -f "$file_path" ]]; then
      host=$(tr -d '[:space:]' < "$file_path")
      if [[ -n "$host" ]]; then
        printf '%s\t%s\n' "$host" "$file_path"
        return 0
      fi
    fi
  done
  return 1
}

resolve_server_target() {
  local discovery host source_path discovered_port

  if [[ -n "$SERVER_HOST" ]]; then
    SERVER_HOST_SOURCE=env
  fi

  if [[ -n "$SERVER_PORT" ]]; then
    SERVER_PORT_SOURCE=env
  fi

  if [[ -z "$SERVER_HOST" ]]; then
    discovery=$(discover_server_host_from_extip || true)
    if [[ -n "$discovery" ]]; then
      IFS=$'\t' read -r host source_path <<< "$discovery"
      SERVER_HOST=$host
      SERVER_HOST_SOURCE="extip:${source_path##*/}"
    fi
  fi

  if [[ -z "$SERVER_HOST" || -z "$SERVER_PORT" ]]; then
    case "$RUNNER_MODE" in
      vagrant-docker)
        discovery=$(discover_service_vagrant)
        if [[ -z "$discovery" ]]; then
          discovery=$(discover_service_local)
        fi
        ;;
      local|docker)
        discovery=$(discover_service_local)
        if [[ -z "$discovery" ]]; then
          discovery=$(discover_service_vagrant)
        fi
        ;;
      *)
        fail "Unsupported RUNNER_MODE for service discovery: $RUNNER_MODE"
        ;;
    esac

    if [[ -n "$discovery" ]]; then
      IFS=$'\t' read -r DISCOVERED_SERVICE_NAMESPACE DISCOVERED_SERVICE_NAME host discovered_port <<< "$discovery"
      if [[ -z "$SERVER_HOST" ]]; then
        SERVER_HOST=$host
        SERVER_HOST_SOURCE="service:${DISCOVERED_SERVICE_NAMESPACE}/${DISCOVERED_SERVICE_NAME}"
      fi
      if [[ -z "$SERVER_PORT" && -n "$discovered_port" ]]; then
        SERVER_PORT=$discovered_port
        SERVER_PORT_SOURCE="service:${DISCOVERED_SERVICE_NAMESPACE}/${DISCOVERED_SERVICE_NAME}"
      fi
    fi
  fi

  if [[ -z "$SERVER_HOST" ]]; then
    fail "SERVER_HOST is empty. Set SERVER_HOST directly, create an extIP file, or expose a discoverable SCTP LoadBalancer service."
  fi

  if [[ -z "$SERVER_PORT" ]]; then
    SERVER_PORT=$DEFAULT_SERVER_PORT
    SERVER_PORT_SOURCE=default
  fi
}

resolve_route_target() {
  local target=$SERVER_HOST
  if [[ "$target" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "$target"
    return
  fi

  if command -v getent >/dev/null 2>&1; then
    local resolved
    resolved=$(getent ahostsv4 "$target" 2>/dev/null | awk 'NR == 1 { print $1 }')
    if [[ -n "$resolved" ]]; then
      echo "$resolved"
      return
    fi
  fi

  echo "$target"
}

discover_source_ip_from_route_local() {
  local route_target=$1
  command -v ip >/dev/null 2>&1 || return 0
  ip -o route get "$route_target" 2>/dev/null | awk '{
    for (i = 1; i <= NF; i++) {
      if ($i == "src" && (i + 1) <= NF) {
        print $(i + 1)
        exit
      }
    }
  }' | head -n 1
}

discover_default_source_ip_local() {
  command -v ip >/dev/null 2>&1 || return 0
  local default_dev
  default_dev=$(ip -o route show default 2>/dev/null | awk 'NR == 1 {
    for (i = 1; i <= NF; i++) {
      if ($i == "dev" && (i + 1) <= NF) {
        print $(i + 1)
        exit
      }
    }
  }')
  if [[ -z "$default_dev" ]]; then
    return 0
  fi
  ip -o -4 addr show dev "$default_dev" scope global 2>/dev/null | awk 'NR == 1 {
    split($4, addr, "/")
    print addr[1]
  }'
}

discover_source_ip_from_route_vagrant() {
  local route_target=$1
  command -v vagrant >/dev/null 2>&1 || return 0
  local quoted_target
  quoted_target=$(printf '%q' "$route_target")
  vagrant ssh "$CLIENT_VM" -c "target=$quoted_target; ip -o route get \"\$target\" 2>/dev/null | awk '{ for (i = 1; i <= NF; i++) if (\$i == \"src\" && (i + 1) <= NF) { print \$(i + 1); exit } }' | head -n 1" 2>/dev/null | tr -d '\r' | tail -n 1
}

discover_default_source_ip_vagrant() {
  command -v vagrant >/dev/null 2>&1 || return 0
  vagrant ssh "$CLIENT_VM" -c "default_dev=\$(ip -o route show default 2>/dev/null | awk 'NR == 1 { for (i = 1; i <= NF; i++) if (\$i == \"dev\" && (i + 1) <= NF) { print \$(i + 1); exit } }'); if [[ -n \"\$default_dev\" ]]; then ip -o -4 addr show dev \"\$default_dev\" scope global 2>/dev/null | awk 'NR == 1 { split(\$4, addr, \"/\"); print addr[1] }'; fi" 2>/dev/null | tr -d '\r' | tail -n 1
}

resolve_host_ip_avp() {
  local route_target auto_ip

  if [[ "$MODE" != "diameter" ]]; then
    return
  fi

  if [[ -n "$HOST_IP_AVP" ]]; then
    HOST_IP_AVP_SOURCE=env
    return
  fi

  route_target=$(resolve_route_target)
  case "$RUNNER_MODE" in
    vagrant-docker)
      auto_ip=$(discover_source_ip_from_route_vagrant "$route_target")
      if [[ -z "$auto_ip" ]]; then
        auto_ip=$(discover_default_source_ip_vagrant)
      fi
      ;;
    local|docker)
      auto_ip=$(discover_source_ip_from_route_local "$route_target")
      if [[ -z "$auto_ip" ]]; then
        auto_ip=$(discover_default_source_ip_local)
      fi
      ;;
    *)
      fail "Unsupported RUNNER_MODE for source IP discovery: $RUNNER_MODE"
      ;;
  esac

  if [[ -z "$auto_ip" ]]; then
    fail "HOST_IP_AVP is empty. Set HOST_IP_AVP directly or ensure the runner can resolve a source IP for SERVER_HOST=$SERVER_HOST."
  fi

  HOST_IP_AVP=$auto_ip
  HOST_IP_AVP_SOURCE=auto
}

print_context() {
  log "Scenario          : k3s-sctp-teardown-pysctp"
  log "Runner mode       : ${RUNNER_MODE}"
  log "Protocol mode     : ${MODE}"
  log "Server host       : ${SERVER_HOST} (${SERVER_HOST_SOURCE:-set})"
  log "Server port       : ${SERVER_PORT} (${SERVER_PORT_SOURCE:-set})"
  if [[ -n "$DISCOVERED_SERVICE_NAME" ]]; then
    log "Service target    : ${DISCOVERED_SERVICE_NAMESPACE}/${DISCOVERED_SERVICE_NAME}"
  fi
  if [[ "$MODE" == "diameter" ]]; then
    log "Host IP AVP       : ${HOST_IP_AVP} (${HOST_IP_AVP_SOURCE:-set})"
  fi
  log "Attempts          : ${ATTEMPTS}"
  log "Idle seconds      : ${IDLE_SECONDS}"
  log "Idle jitter       : ${IDLE_JITTER}"
  log "Idle mode         : ${IDLE_MODE}"
  log "Close mode        : ${CLOSE_MODE}"
  log "Artifacts         : ${ARTIFACT_DIR}"
}

check_local_sctp_support() {
  ensure_cmd python3
  local rc
  set +e
  python3 - <<'PY'
import socket
import sys
proto = getattr(socket, "IPPROTO_SCTP", None)
if proto is None:
    sys.exit(2)
try:
    socket.socket(socket.AF_INET, socket.SOCK_STREAM, proto).close()
except OSError:
    sys.exit(1)
sys.exit(0)
PY
  rc=$?
  set -e
  case "$rc" in
    0)
      return
      ;;
    1)
      fail "Kernel SCTP socket creation failed in the current execution environment."
      ;;
    2)
      fail "This Python runtime does not expose IPPROTO_SCTP."
      ;;
    *)
      fail "Unexpected SCTP capability check failure."
      ;;
  esac
}

build_python_args() {
  local summary_target=$1
  local event_target=$2
  local args=()
  args+=(--server-host "$SERVER_HOST")
  args+=(--server-port "$SERVER_PORT")
  args+=(--mode "$MODE")
  args+=(--attempts "$ATTEMPTS")
  args+=(--idle-seconds "$IDLE_SECONDS")
  args+=(--idle-jitter "$IDLE_JITTER")
  args+=(--idle-mode "$IDLE_MODE")
  args+=(--dwr-interval "$DWR_INTERVAL")
  args+=(--between-attempts "$BETWEEN_ATTEMPTS")
  args+=(--origin-host "$ORIGIN_HOST")
  args+=(--origin-realm "$ORIGIN_REALM")
  args+=(--product-name "$PRODUCT_NAME")
  args+=(--vendor-id "$VENDOR_ID")
  args+=(--disconnect-cause "$DISCONNECT_CAUSE")
  args+=(--close-mode "$CLOSE_MODE")
  args+=(--summary-path "$summary_target")
  args+=(--event-log-path "$event_target")

  if [[ -n "$HOST_IP_AVP" ]]; then
    args+=(--host-ip-avp "$HOST_IP_AVP")
  fi
  if [[ -n "$DESTINATION_HOST" ]]; then
    args+=(--destination-host "$DESTINATION_HOST")
  fi
  if [[ -n "$DESTINATION_REALM" ]]; then
    args+=(--destination-realm "$DESTINATION_REALM")
  fi

  local dpa_flag dwa_flag fail_flag
  dpa_flag=$(bool_arg "$WAIT_FOR_DPA" --wait-for-dpa)
  dwa_flag=$(bool_arg "$WAIT_FOR_DWA" --wait-for-dwa)
  fail_flag=$(bool_arg "$FAIL_ON_ATTEMPT_ERROR" --fail-on-attempt-error)

  if [[ -n "$dpa_flag" ]]; then
    args+=("$dpa_flag")
  fi
  if [[ -n "$dwa_flag" ]]; then
    args+=("$dwa_flag")
  fi
  if [[ -n "$fail_flag" ]]; then
    args+=("$fail_flag")
  fi

  printf '%q ' "${args[@]}"
}

run_local_python() {
  check_local_sctp_support
  local args
  args=$(build_python_args "$SUMMARY_PATH" "$EVENT_LOG_PATH")
  log "Running Python harness locally"
  bash -lc "cd '$SCRIPT_DIR' && python3 diameter_dpr_loop.py $args" | tee "$STDOUT_LOG_PATH"
}

run_local_docker() {
  ensure_cmd docker
  log "Checking local SCTP support before docker run"
  check_local_sctp_support
  if [[ "$REBUILD_IMAGE" == "yes" || -z "$(docker images -q "$LOCAL_IMAGE" 2>/dev/null)" ]]; then
    log "Building local docker image: $LOCAL_IMAGE"
    docker build -t "$LOCAL_IMAGE" "$SCRIPT_DIR" >/dev/null
  fi
  local args
  args=$(build_python_args "$CONTAINER_SUMMARY_PATH" "$CONTAINER_EVENT_LOG_PATH")
  log "Running Python harness in local docker"
  bash -lc "docker run --rm --network host -v '$ARTIFACT_DIR:/artifacts' '$LOCAL_IMAGE' $args" | tee "$STDOUT_LOG_PATH"
}

run_vagrant_docker() {
  ensure_cmd vagrant
  local args remote_cmd
  args=$(build_python_args "$CONTAINER_SUMMARY_PATH" "$CONTAINER_EVENT_LOG_PATH")
  remote_cmd="set -euo pipefail; \
    cd /vagrant/cicd/k3s-sctp-teardown-pysctp; \
    set +e; \
    python3 - <<'PY'
import socket
import sys
proto = getattr(socket, 'IPPROTO_SCTP', None)
if proto is None:
    sys.exit(2)
try:
    socket.socket(socket.AF_INET, socket.SOCK_STREAM, proto).close()
except OSError:
    sys.exit(1)
sys.exit(0)
PY
    rc=\$?; \
    set -e; \
    if [[ \$rc -eq 1 ]]; then echo 'Kernel SCTP socket creation failed on $CLIENT_VM'; exit 41; fi; \
    if [[ \$rc -eq 2 ]]; then echo 'Python runtime on $CLIENT_VM does not expose IPPROTO_SCTP'; exit 42; fi; \
    if [[ '$REBUILD_IMAGE' == 'yes' ]] || [[ -z \$(sudo docker images -q '$LOCAL_IMAGE' 2>/dev/null) ]]; then \
      sudo docker build -t '$LOCAL_IMAGE' . >/dev/null; \
    fi; \
    mkdir -p /vagrant/cicd/k3s-sctp-teardown-pysctp/artifacts/$TIMESTAMP; \
    sudo docker run --rm --network host \
      -v /vagrant/cicd/k3s-sctp-teardown-pysctp/artifacts/$TIMESTAMP:/artifacts \
      '$LOCAL_IMAGE' \
      $args"
  log "Running Python harness in docker on vagrant VM: $CLIENT_VM"
  vagrant ssh "$CLIENT_VM" -c "$remote_cmd" 2>/dev/null | tee "$STDOUT_LOG_PATH"
}

stage_from_summary() {
  python3 - "$SUMMARY_PATH" "$MODE" "$WAIT_FOR_DPA" "$CLOSE_MODE" "$REQUIRE_PEER_CLOSE" <<'PY'
import json
import sys

summary_path, mode, wait_for_dpa, close_mode, require_peer_close = sys.argv[1:]
wait_for_dpa = wait_for_dpa.lower() in {"yes", "true", "1"}
require_peer_close = require_peer_close.lower() in {"yes", "true", "1"}

with open(summary_path, encoding="utf-8") as stream:
    data = json.load(stream)

failures = []
for entry in data:
    stage = None
    reason = None

    if entry.get("error"):
        error = str(entry["error"])
        if "command 257" in error:
            stage = "CER/CEA"
        elif "command 280" in error:
            stage = "DWR/DWA"
        elif "command 282" in error:
            stage = "DPR/DPA"
        elif "peer closed connection" in error:
            stage = "Peer Close"
        else:
            stage = "Runtime"
        reason = error
    elif not entry.get("connected"):
        stage = "Connect"
        reason = "socket connection was not established"
    elif mode == "diameter" and not entry.get("cer_sent"):
        stage = "CER"
        reason = "CER was not sent"
    elif mode == "diameter" and not entry.get("cea_ok"):
        stage = "CEA"
        reason = "CEA was not received successfully"
    elif mode == "diameter" and not entry.get("dpr_sent"):
        stage = "DPR"
        reason = "DPR was not sent"
    elif mode == "diameter" and wait_for_dpa and not entry.get("dpa_ok"):
        stage = "DPA"
        reason = "DPA was not received successfully"
    elif close_mode == "client" and not entry.get("local_shutdown"):
        stage = "SCTP Shutdown"
        reason = "local SCTP shutdown was not triggered"
    elif require_peer_close and not entry.get("peer_closed"):
        stage = "Peer Close"
        reason = "peer did not close after shutdown within timeout"

    if stage is not None:
        failures.append((entry.get("attempt"), stage, reason))

if failures:
    for attempt, stage, reason in failures:
        print(f"ATTEMPT={attempt}\tSTAGE={stage}\tREASON={reason}")
    sys.exit(1)

print(f"ATTEMPTS_OK={len(data)}")
PY
}

print_failure_logs() {
  log "Validation artifacts"
  log "  summary : $SUMMARY_PATH"
  log "  events  : $EVENT_LOG_PATH"
  log "  runner  : $STDOUT_LOG_PATH"

  if [[ -f "$STDOUT_LOG_PATH" ]]; then
    echo "---- runner log (tail) ----"
    tail -n 40 "$STDOUT_LOG_PATH"
  fi
  if [[ -f "$EVENT_LOG_PATH" ]]; then
    echo "---- event log (tail) ----"
    tail -n 40 "$EVENT_LOG_PATH"
  fi
}

resolve_server_target
resolve_host_ip_avp
print_context

set +e
case "$RUNNER_MODE" in
  local)
    run_local_python
    ;;
  docker)
    run_local_docker
    ;;
  vagrant-docker)
    run_vagrant_docker
    ;;
  *)
    fail "Unsupported RUNNER_MODE: $RUNNER_MODE"
    ;;
esac
runner_rc=$?
set -e

if [[ $runner_rc -ne 0 ]]; then
  log "Runner exited with code $runner_rc"
fi

if [[ ! -f "$SUMMARY_PATH" ]]; then
  echo "k3s-sctp-teardown-pysctp validation [NOK]"
  echo "ATTEMPT=unknown	STAGE=Runner	REASON=summary file was not produced: $SUMMARY_PATH"
  print_failure_logs
  exit 1
fi

set +e
failure_report=$(stage_from_summary)
stage_rc=$?
set -e

if [[ $stage_rc -eq 0 ]]; then
  echo "k3s-sctp-teardown-pysctp validation [OK]"
  echo "$failure_report"
  exit 0
fi

echo "k3s-sctp-teardown-pysctp validation [NOK]"
echo "$failure_report"
print_failure_logs
exit 1