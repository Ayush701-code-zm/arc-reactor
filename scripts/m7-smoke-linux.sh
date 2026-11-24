#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ "$(uname -s)" != "Linux" ]]; then
  printf 'This smoke test needs Linux (use scripts/dev-ubuntu-docker.sh).\n' >&2
  exit 1
fi

BUILD_DIR="${TINY_RUNTIME_BUILD_DIR:-$ROOT/runtime/build-linux}"
EBPF_BUILD="${TINY_EBPF_BUILD_DIR:-$ROOT/observability/tinyebpf/build-linux}"
mkdir -p data control-plane/bin "$BUILD_DIR" "$EBPF_BUILD"

(
  cd control-plane
  go test ./...
  go build -o bin/tiny ./cmd/tiny
  go build -o bin/tinyd ./cmd/tinyd
)

cmake -S runtime -B "$BUILD_DIR"
cmake --build "$BUILD_DIR"
ctest --test-dir "$BUILD_DIR" --output-on-failure

cmake -S observability/tinyebpf -B "$EBPF_BUILD"
cmake --build "$EBPF_BUILD"

export TINY_DB="${TINY_M7_DB:-$ROOT/data/tiny-m7.db}"
export TINY_RUNTIME_ADDR="${TINY_M7_RUNTIME_ADDR:-127.0.0.1:15055}"
export TINY_API_ADDR="${TINY_M7_API_ADDR:-127.0.0.1:18084}"
export TINY_IMAGE_ROOT="${TINY_M7_IMAGE_ROOT:-$ROOT/data/images-m7}"
export TINY_VOLUME_ROOT="${TINY_M7_VOLUME_ROOT:-$ROOT/data/volumes-m7}"
export TINY_EBPF_SOCK="${TINY_M7_EBPF_SOCK:-/tmp/tiny-aws-ebpf-m7.sock}"
export TINY_EBPF_OBJECT_DIR="$EBPF_BUILD"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"
export TINY_REQUIRE_EBPF="${TINY_REQUIRE_EBPF:-0}"
mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT"
rm -f "$TINY_DB" "$TINY_EBPF_SOCK"

ARCH=$(uname -m)
case "$ARCH" in aarch64|arm64) A=aarch64;; *) A=x86_64;; esac
ALPINE_TGZ="/tmp/alpine-minirootfs-3.20.3-${A}.tar.gz"
if [[ ! -f "$ALPINE_TGZ" ]]; then
  curl -fsSL --proto '=https' --proto-redir '=https' -o "$ALPINE_TGZ" \
    "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/${A}/alpine-minirootfs-3.20.3-${A}.tar.gz"
fi

"$EBPF_BUILD/tinyebpf" >/tmp/tinyebpf-m7.log 2>&1 &
EPID=$!
"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-m7.log 2>&1 &
RPID=$!
control-plane/bin/tinyd >/tmp/tinyd-m7.log 2>&1 &
DPID=$!
cleanup() {
  kill "$DPID" "$RPID" "$EPID" 2>/dev/null || true
  wait "$DPID" "$RPID" "$EPID" 2>/dev/null || true
}
trap cleanup EXIT

for i in 1 2 3 4 5 6 7 8 9 10; do
  if ! kill -0 "$DPID" 2>/dev/null || ! kill -0 "$RPID" 2>/dev/null || ! kill -0 "$EPID" 2>/dev/null; then
    printf 'daemon exited early\n' >&2
    cat /tmp/tinyebpf-m7.log >&2 || true
    cat /tmp/tinyd-m7.log >&2 || true
    exit 1
  fi
  curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null && break
  if [[ "$i" -eq 10 ]]; then
    printf 'tinyd not ready\n' >&2
    cat /tmp/tinyd-m7.log >&2 || true
    cat /tmp/tinyebpf-m7.log >&2 || true
    exit 1
  fi
  sleep 0.3
done

TINY=control-plane/bin/tiny
"$TINY" auth bootstrap --username admin --password adminadmin || true
LOGIN_JSON="$("$TINY" auth login --username admin --password adminadmin)"
export TINY_TOKEN="$(printf '%s' "$LOGIN_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["AccessToken"])')"

IMG=m7-alpine
NAME=m7-web
"$TINY" image import --name "$IMG" --file "$ALPINE_TGZ" >/dev/null
"$TINY" instance create --name "$NAME" --image "$IMG" --cpu 1 --memory 64MB >/dev/null

INS="$("$TINY" instance inspect "$NAME")"
printf '%s\n' "$INS"
printf '%s' "$INS" | python3 -c 'import sys,json; i=json.load(sys.stdin); assert i.get("Observability") is not None, i; o=i["Observability"]; assert "CPUUsec" in o and "MemoryBytes" in o, o'

"$TINY" instance delete "$NAME" >/dev/null
"$TINY" image delete "$IMG" >/dev/null
printf 'M7 smoke passed\n'
