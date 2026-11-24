#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ "$(uname -s)" != "Linux" ]]; then
  printf 'This smoke test needs Linux (use scripts/dev-ubuntu-docker.sh).\n' >&2
  exit 1
fi

BUILD_DIR="${TINY_RUNTIME_BUILD_DIR:-$ROOT/runtime/build-linux}"
mkdir -p data control-plane/bin "$BUILD_DIR"

(
  cd control-plane
  go test ./...
  go build -o bin/tiny ./cmd/tiny
  go build -o bin/tinyd ./cmd/tinyd
)

cmake -S runtime -B "$BUILD_DIR"
cmake --build "$BUILD_DIR" -j"$(nproc 2>/dev/null || echo 4)"
ctest --test-dir "$BUILD_DIR" --output-on-failure

export TINY_DB="${TINY_M8_DB:-$ROOT/data/tiny-m8.db}"
export TINY_RUNTIME_ADDR="${TINY_M8_RUNTIME_ADDR:-127.0.0.1:15056}"
export TINY_NODE_CONTROL_ADDR="${TINY_M8_NODE_CONTROL_ADDR:-127.0.0.1:15057}"
export TINY_API_ADDR="${TINY_M8_API_ADDR:-127.0.0.1:18085}"
export TINY_IMAGE_ROOT="${TINY_M8_IMAGE_ROOT:-$ROOT/data/images-m8}"
export TINY_VOLUME_ROOT="${TINY_M8_VOLUME_ROOT:-$ROOT/data/volumes-m8}"
export TINY_NODE_ID_FILE="${TINY_M8_NODE_ID_FILE:-/tmp/tiny-node-id-m8}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"
mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT"
rm -f "$TINY_DB" "$TINY_NODE_ID_FILE"

ARCH=$(uname -m)
case "$ARCH" in aarch64|arm64) A=aarch64;; *) A=x86_64;; esac
ALPINE_TGZ="/tmp/alpine-minirootfs-3.20.3-${A}.tar.gz"
if [[ ! -f "$ALPINE_TGZ" ]]; then
  curl -fsSL --proto '=https' --proto-redir '=https' -o "$ALPINE_TGZ" \
    "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/${A}/alpine-minirootfs-3.20.3-${A}.tar.gz"
fi

"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-m8.log 2>&1 &
RPID=$!
control-plane/bin/tinyd >/tmp/tinyd-m8.log 2>&1 &
DPID=$!
"$BUILD_DIR/tinynodeagent" --control "$TINY_NODE_CONTROL_ADDR" --runtime "$TINY_RUNTIME_ADDR" \
  --node-id-file "$TINY_NODE_ID_FILE" >/tmp/tinynodeagent-m8.log 2>&1 &
APID=$!
cleanup() {
  kill "$DPID" "$RPID" "$APID" 2>/dev/null || true
  wait "$DPID" "$RPID" "$APID" 2>/dev/null || true
}
trap cleanup EXIT

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  if ! kill -0 "$DPID" 2>/dev/null || ! kill -0 "$RPID" 2>/dev/null || ! kill -0 "$APID" 2>/dev/null; then
    printf 'daemon exited early\n' >&2
    cat /tmp/tinyd-m8.log >&2 || true
    cat /tmp/tinynodeagent-m8.log >&2 || true
    exit 1
  fi
  curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null && break
  if [[ "$i" -eq 15 ]]; then
    printf 'tinyd not ready\n' >&2
    cat /tmp/tinyd-m8.log >&2 || true
    exit 1
  fi
  sleep 0.3
done

TINY=control-plane/bin/tiny
"$TINY" auth bootstrap --username admin --password adminadmin || true
LOGIN_JSON="$("$TINY" auth login --username admin --password adminadmin)"
export TINY_TOKEN="$(printf '%s' "$LOGIN_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["AccessToken"])')"

wait_node_py=$(cat <<'PY'
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(1)
if isinstance(d, list) and len(d) >= 1 and d[0].get("Connected"):
    print(d[0]["ID"])
    sys.exit(0)
sys.exit(1)
PY
)

NODE_ID=""
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  NODES="$("$TINY" node list 2>/dev/null || true)"
  NODE_ID="$(printf '%s' "$NODES" | python3 -c "$wait_node_py" 2>/dev/null || true)"
  if [[ -n "$NODE_ID" ]]; then
    break
  fi
  if [[ "$i" -eq 15 ]]; then
    printf 'node agent did not register\n' >&2
    cat /tmp/tinynodeagent-m8.log >&2 || true
    cat /tmp/tinyd-m8.log >&2 || true
    exit 1
  fi
  sleep 0.3
done
printf 'registered node %s\n' "$NODE_ID"

IMG=m8-alpine
NAME=m8-web
"$TINY" image import --name "$IMG" --file "$ALPINE_TGZ" >/dev/null
"$TINY" instance create --name "$NAME" --image "$IMG" --cpu 1 --memory 64MB >/dev/null

INS="$("$TINY" instance inspect "$NAME")"
printf '%s\n' "$INS"
check_py=$(cat <<'PY'
import json, sys
i = json.load(sys.stdin)
assert i.get("NodeID"), i
assert i["Status"] == "RUNNING", i
assert i["NodeID"] != "local", i
PY
)
printf '%s' "$INS" | python3 -c "$check_py"

"$TINY" instance delete "$NAME" >/dev/null
"$TINY" image delete "$IMG" >/dev/null
printf 'M8 smoke passed\n'
