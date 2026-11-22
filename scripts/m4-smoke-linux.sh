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

printf '== build Go ==\n'
(
  cd control-plane
  go test ./...
  go build -o bin/tiny ./cmd/tiny
  go build -o bin/tinyd ./cmd/tinyd
)

printf '== build C++ (%s) ==\n' "$BUILD_DIR"
cmake -S runtime -B "$BUILD_DIR"
cmake --build "$BUILD_DIR"
ctest --test-dir "$BUILD_DIR" --output-on-failure

export TINY_DB="${TINY_M4_DB:-$ROOT/data/tiny-m4.db}"
export TINY_RUNTIME_ADDR="${TINY_M4_RUNTIME_ADDR:-127.0.0.1:15052}"
export TINY_API_ADDR="${TINY_M4_API_ADDR:-127.0.0.1:18081}"
export TINY_IMAGE_ROOT="${TINY_M4_IMAGE_ROOT:-$ROOT/data/images-m4}"
export TINY_VOLUME_ROOT="${TINY_M4_VOLUME_ROOT:-$ROOT/data/volumes-m4}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"
mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT"
rm -f "$TINY_DB"
printf 'M4 endpoints: api=%s runtime=%s db=%s\n' "$TINY_API_ADDR" "$TINY_RUNTIME_ADDR" "$TINY_DB"

ARCH="$(uname -m)"
case "$ARCH" in
  aarch64|arm64) ALPINE_ARCH=aarch64 ;;
  x86_64|amd64) ALPINE_ARCH=x86_64 ;;
  *) printf 'unsupported arch %s\n' "$ARCH" >&2; exit 1 ;;
esac
ALPINE_VER="${ALPINE_VER:-3.20.3}"
ALPINE_TGZ="/tmp/alpine-minirootfs-${ALPINE_VER}-${ALPINE_ARCH}.tar.gz"
ALPINE_URL="https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/${ALPINE_ARCH}/alpine-minirootfs-${ALPINE_VER}-${ALPINE_ARCH}.tar.gz"

printf '== fetch alpine minirootfs ==\n'
if [[ ! -f "$ALPINE_TGZ" ]]; then
  curl -fsSL --proto '=https' --proto-redir '=https' -o "$ALPINE_TGZ" "$ALPINE_URL"
fi

printf '== start tinyruntimed on %s ==\n' "$TINY_RUNTIME_ADDR"
"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-m4.log 2>&1 &
RPID=$!

printf '== start tinyd (http://%s) ==\n' "$TINY_API_ADDR"
control-plane/bin/tinyd >/tmp/tinyd-m4.log 2>&1 &
DPID=$!

cleanup() {
  if kill -0 "$DPID" 2>/dev/null; then
    kill "$DPID" 2>/dev/null || true
    wait "$DPID" 2>/dev/null || true
  fi
  if kill -0 "$RPID" 2>/dev/null; then
    kill "$RPID" 2>/dev/null || true
    wait "$RPID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

for i in 1 2 3 4 5 6 7 8 9 10; do
  if ! kill -0 "$DPID" 2>/dev/null || ! kill -0 "$RPID" 2>/dev/null; then
    printf 'daemon exited early\n' >&2
    cat /tmp/tinyd-m4.log >&2 || true
    cat /tmp/tinyruntimed-m4.log >&2 || true
    exit 1
  fi
  if curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null; then
    break
  fi
  if [[ "$i" -eq 10 ]]; then
    printf 'tinyd did not become ready on %s\n' "$TINY_API_ADDR" >&2
    cat /tmp/tinyd-m4.log >&2 || true
    cat /tmp/tinyruntimed-m4.log >&2 || true
    exit 1
  fi
  sleep 0.3
done

TINY=control-plane/bin/tiny
IMG="m4-alpine"
NAME="m4-inst-$(date +%s)"
NET="m4-net"

printf '== image import ==\n'
"$TINY" image import --name "$IMG" --file "$ALPINE_TGZ"

printf '== network create ==\n'
NET_JSON="$("$TINY" network create --name "$NET" --cidr 10.50.0.0/24)"
printf '%s\n' "$NET_JSON"
printf '%s' "$NET_JSON" | python3 -c 'import sys,json; n=json.load(sys.stdin); assert n["GatewayIP"]=="10.50.0.1" and n["Status"]=="READY", n'

printf '== instance create with --network ==\n'
CREATE_JSON="$("$TINY" instance create --name "$NAME" --image "$IMG" --cpu 1 --memory 64MB --network "$NET")"
printf '%s\n' "$CREATE_JSON"
printf '%s' "$CREATE_JSON" | python3 -c 'import sys,json; i=json.load(sys.stdin); assert i["Status"]=="RUNNING", i'

printf '== network inspect ==\n'
"$TINY" network inspect "$NET"

printf '== network detach ==\n'
"$TINY" network detach --network "$NET" --instance "$NAME"

printf '== network attach again ==\n'
ATT_JSON="$("$TINY" network attach --network "$NET" --instance "$NAME")"
printf '%s\n' "$ATT_JSON"
printf '%s' "$ATT_JSON" | python3 -c 'import sys,json; a=json.load(sys.stdin); assert a["GuestIP"]=="10.50.0.2" and a["Status"]=="ATTACHED", a'

printf '== instance delete (auto detach) ==\n'
"$TINY" instance delete "$NAME"

printf '== network delete ==\n'
"$TINY" network delete "$NET"

printf '== image delete ==\n'
"$TINY" image delete "$IMG"

printf 'M4 smoke passed for network=%s instance=%s\n' "$NET" "$NAME"
