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

export TINY_DB="${TINY_V21_DB:-$ROOT/data/tiny-v21.db}"
export TINY_RUNTIME_ADDR="${TINY_V21_RUNTIME_ADDR:-127.0.0.1:15061}"
export TINY_NODE_CONTROL_ADDR="${TINY_V21_NODE_CONTROL_ADDR:-127.0.0.1:15062}"
export TINY_API_ADDR="${TINY_V21_API_ADDR:-127.0.0.1:18091}"
export TINY_IMAGE_ROOT="${TINY_V21_IMAGE_ROOT:-$ROOT/data/images-v21}"
export TINY_VOLUME_ROOT="${TINY_V21_VOLUME_ROOT:-$ROOT/data/volumes-v21}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"

NODE_A_ID_FILE=/tmp/tiny-node-id-v21-a
NODE_B_ID_FILE=/tmp/tiny-node-id-v21-b
IMAGE_ROOT_A="$ROOT/data/node-images-v21-a"
IMAGE_ROOT_B="$ROOT/data/node-images-v21-b"
INST_ROOT_A="$ROOT/data/node-instances-v21-a"
INST_ROOT_B="$ROOT/data/node-instances-v21-b"

mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT" "$IMAGE_ROOT_A" "$IMAGE_ROOT_B" "$INST_ROOT_A" "$INST_ROOT_B"
rm -f "$TINY_DB" "$NODE_A_ID_FILE" "$NODE_B_ID_FILE"
rm -rf "$IMAGE_ROOT_A"/* "$IMAGE_ROOT_B"/* "$INST_ROOT_A"/* "$INST_ROOT_B"/* 2>/dev/null || true

ARCH=$(uname -m)
case "$ARCH" in aarch64|arm64) A=aarch64;; *) A=x86_64;; esac
ALPINE_TGZ="/tmp/alpine-minirootfs-3.20.3-${A}.tar.gz"
if [[ ! -f "$ALPINE_TGZ" ]]; then
  curl -fsSL --proto '=https' --proto-redir '=https' -o "$ALPINE_TGZ" \
    "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/${A}/alpine-minirootfs-3.20.3-${A}.tar.gz"
fi

"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-v21.log 2>&1 &
RPID=$!
control-plane/bin/tinyd >/tmp/tinyd-v21.log 2>&1 &
DPID=$!

TINY_NODE_IMAGE_ROOT="$IMAGE_ROOT_A" TINY_NODE_INSTANCE_ROOT="$INST_ROOT_A" \
  "$BUILD_DIR/tinynodeagent" --control "$TINY_NODE_CONTROL_ADDR" --runtime "$TINY_RUNTIME_ADDR" \
  --node-id-file "$NODE_A_ID_FILE" --image-root "$IMAGE_ROOT_A" --instance-root "$INST_ROOT_A" \
  >/tmp/tinynodeagent-v21-a.log 2>&1 &
APID=$!

TINY_NODE_IMAGE_ROOT="$IMAGE_ROOT_B" TINY_NODE_INSTANCE_ROOT="$INST_ROOT_B" \
  "$BUILD_DIR/tinynodeagent" --control "$TINY_NODE_CONTROL_ADDR" --runtime "$TINY_RUNTIME_ADDR" \
  --node-id-file "$NODE_B_ID_FILE" --image-root "$IMAGE_ROOT_B" --instance-root "$INST_ROOT_B" \
  >/tmp/tinynodeagent-v21-b.log 2>&1 &
BPID=$!

cleanup() {
  kill "$DPID" "$RPID" "$APID" "$BPID" 2>/dev/null || true
  wait "$DPID" "$RPID" "$APID" "$BPID" 2>/dev/null || true
}
trap cleanup EXIT

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  if ! kill -0 "$DPID" 2>/dev/null || ! kill -0 "$RPID" 2>/dev/null; then
    printf 'daemon exited early\n' >&2
    cat /tmp/tinyd-v21.log >&2 || true
    exit 1
  fi
  curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null && break
  if [[ "$i" -eq 15 ]]; then
    printf 'tinyd not ready\n' >&2
    cat /tmp/tinyd-v21.log >&2 || true
    exit 1
  fi
  sleep 0.3
done

TINY=control-plane/bin/tiny
"$TINY" auth bootstrap --username admin --password adminadmin || true
LOGIN_JSON="$("$TINY" auth login --username admin --password adminadmin)"
export TINY_TOKEN="$(printf '%s' "$LOGIN_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["AccessToken"])')"

wait_two_py=$(cat <<'PY'
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(1)
if not isinstance(d, list) or len(d) < 2:
    sys.exit(1)
ids = [n["ID"] for n in d if n.get("Connected")]
if len(ids) < 2:
    sys.exit(1)
print(ids[0])
print(ids[1])
PY
)

NODE_A=""
NODE_B=""
for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do
  NODES="$("$TINY" node list 2>/dev/null || true)"
  PAIR="$(printf '%s' "$NODES" | python3 -c "$wait_two_py" 2>/dev/null || true)"
  if [[ -n "$PAIR" ]]; then
    NODE_A="$(printf '%s\n' "$PAIR" | sed -n '1p')"
    NODE_B="$(printf '%s\n' "$PAIR" | sed -n '2p')"
    break
  fi
  if [[ "$i" -eq 20 ]]; then
    printf 'expected two connected agents\n' >&2
    cat /tmp/tinynodeagent-v21-a.log >&2 || true
    cat /tmp/tinynodeagent-v21-b.log >&2 || true
    cat /tmp/tinyd-v21.log >&2 || true
    exit 1
  fi
  sleep 0.3
done
printf 'nodes A=%s B=%s\n' "$NODE_A" "$NODE_B"

# Map file-based node ids to list order for place targeting.
FILE_A="$(tr -d '\n' < "$NODE_A_ID_FILE")"
FILE_B="$(tr -d '\n' < "$NODE_B_ID_FILE")"
PLACE_NODE="$FILE_A"
OTHER_NODE="$FILE_B"
printf 'place only on %s (other=%s)\n' "$PLACE_NODE" "$OTHER_NODE"

IMG=v21-alpine
"$TINY" image import --name "$IMG" --file "$ALPINE_TGZ" >/dev/null
"$TINY" image place --name "$IMG" --node "$PLACE_NODE" >/dev/null

# Stop the node that has the image so first-fit must pick the empty node.
if [[ "$PLACE_NODE" == "$FILE_A" ]]; then
  kill "$APID" 2>/dev/null || true
  wait "$APID" 2>/dev/null || true
  APID=""
else
  kill "$BPID" 2>/dev/null || true
  wait "$BPID" 2>/dev/null || true
  BPID=""
fi

for i in 1 2 3 4 5 6 7 8 9 10; do
  NODES="$("$TINY" node list 2>/dev/null || true)"
  COUNT="$(printf '%s' "$NODES" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sum(1 for n in d if n.get("Connected")))')"
  if [[ "$COUNT" == "1" ]]; then
    break
  fi
  sleep 0.3
done

set +e
FAIL_OUT="$("$TINY" instance create --name v21-miss --image "$IMG" --cpu 1 --memory 64MB 2>&1)"
FAIL_RC=$?
set -e
if [[ "$FAIL_RC" -eq 0 ]]; then
  printf 'expected create to fail on node without image\n%s\n' "$FAIL_OUT" >&2
  exit 1
fi
printf '%s\n' "$FAIL_OUT" | grep -q "not READY on node" || {
  printf 'missing READY error:\n%s\n' "$FAIL_OUT" >&2
  exit 1
}
printf '%s\n' "$FAIL_OUT" | grep -q "$OTHER_NODE" || {
  printf 'error should name node %s:\n%s\n' "$OTHER_NODE" "$FAIL_OUT" >&2
  exit 1
}
printf 'isolation fail path ok\n'

# Bring place-node back; stop the other so create lands on READY node.
if [[ -z "$APID" ]]; then
  TINY_NODE_IMAGE_ROOT="$IMAGE_ROOT_A" TINY_NODE_INSTANCE_ROOT="$INST_ROOT_A" \
    "$BUILD_DIR/tinynodeagent" --control "$TINY_NODE_CONTROL_ADDR" --runtime "$TINY_RUNTIME_ADDR" \
    --node-id-file "$NODE_A_ID_FILE" --image-root "$IMAGE_ROOT_A" --instance-root "$INST_ROOT_A" \
    >/tmp/tinynodeagent-v21-a.log 2>&1 &
  APID=$!
  kill "$BPID" 2>/dev/null || true
  wait "$BPID" 2>/dev/null || true
  BPID=""
else
  TINY_NODE_IMAGE_ROOT="$IMAGE_ROOT_B" TINY_NODE_INSTANCE_ROOT="$INST_ROOT_B" \
    "$BUILD_DIR/tinynodeagent" --control "$TINY_NODE_CONTROL_ADDR" --runtime "$TINY_RUNTIME_ADDR" \
    --node-id-file "$NODE_B_ID_FILE" --image-root "$IMAGE_ROOT_B" --instance-root "$INST_ROOT_B" \
    >/tmp/tinynodeagent-v21-b.log 2>&1 &
  BPID=$!
  kill "$APID" 2>/dev/null || true
  wait "$APID" 2>/dev/null || true
  APID=""
fi

for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  NODES="$("$TINY" node list 2>/dev/null || true)"
  READY="$(printf '%s' "$NODES" | python3 -c "
import json,sys
d=json.load(sys.stdin)
ok=any(n.get('ID')=='$PLACE_NODE' and n.get('Connected') for n in d)
print('1' if ok else '0')
")"
  if [[ "$READY" == "1" ]]; then
    break
  fi
  if [[ "$i" -eq 15 ]]; then
    printf 'place node did not reconnect\n' >&2
    exit 1
  fi
  sleep 0.3
done

"$TINY" instance create --name v21-web --image "$IMG" --cpu 1 --memory 64MB >/dev/null
INS="$("$TINY" instance inspect v21-web)"
printf '%s\n' "$INS"
printf '%s' "$INS" | python3 -c "
import json,sys
i=json.load(sys.stdin)
assert i.get('Status')=='RUNNING', i
assert i.get('NodeID')=='$PLACE_NODE', i
"
"$TINY" instance delete v21-web >/dev/null
printf 'v21 smoke ok\n'
