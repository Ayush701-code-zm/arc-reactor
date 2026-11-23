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
cmake --build "$BUILD_DIR"
ctest --test-dir "$BUILD_DIR" --output-on-failure

export TINY_DB="${TINY_M6_DB:-$ROOT/data/tiny-m6.db}"
export TINY_RUNTIME_ADDR="${TINY_M6_RUNTIME_ADDR:-127.0.0.1:15054}"
export TINY_API_ADDR="${TINY_M6_API_ADDR:-127.0.0.1:18083}"
export TINY_IMAGE_ROOT="${TINY_M6_IMAGE_ROOT:-$ROOT/data/images-m6}"
export TINY_VOLUME_ROOT="${TINY_M6_VOLUME_ROOT:-$ROOT/data/volumes-m6}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"
mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT"
rm -f "$TINY_DB"

ARCH=$(uname -m)
case "$ARCH" in aarch64|arm64) A=aarch64;; *) A=x86_64;; esac
ALPINE_TGZ="/tmp/alpine-minirootfs-3.20.3-${A}.tar.gz"
if [[ ! -f "$ALPINE_TGZ" ]]; then
  curl -fsSL --proto '=https' --proto-redir '=https' -o "$ALPINE_TGZ" \
    "https://dl-cdn.alpinelinux.org/alpine/v3.20/releases/${A}/alpine-minirootfs-3.20.3-${A}.tar.gz"
fi

printf 'M6 endpoints: api=%s runtime=%s db=%s\n' "$TINY_API_ADDR" "$TINY_RUNTIME_ADDR" "$TINY_DB"
"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-m6.log 2>&1 &
RPID=$!
control-plane/bin/tinyd >/tmp/tinyd-m6.log 2>&1 &
DPID=$!
cleanup() {
  kill "$DPID" "$RPID" 2>/dev/null || true
  wait "$DPID" "$RPID" 2>/dev/null || true
}
trap cleanup EXIT

for i in 1 2 3 4 5 6 7 8 9 10; do
  if ! kill -0 "$DPID" 2>/dev/null || ! kill -0 "$RPID" 2>/dev/null; then
    printf 'daemon exited early\n' >&2
    cat /tmp/tinyd-m6.log >&2 || true
    cat /tmp/tinyruntimed-m6.log >&2 || true
    exit 1
  fi
  if curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null; then
    break
  fi
  if [[ "$i" -eq 10 ]]; then
    printf 'tinyd did not become ready on %s\n' "$TINY_API_ADDR" >&2
    cat /tmp/tinyd-m6.log >&2 || true
    cat /tmp/tinyruntimed-m6.log >&2 || true
    exit 1
  fi
  sleep 0.3
done

TINY=control-plane/bin/tiny

printf '== auth bootstrap + login ==\n'
"$TINY" auth bootstrap --username admin --password adminadmin || true
LOGIN_JSON="$("$TINY" auth login --username admin --password adminadmin)"
export TINY_TOKEN="$(printf '%s' "$LOGIN_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["AccessToken"])')"
if [[ -z "${TINY_TOKEN}" ]]; then
  printf 'login failed\n' >&2
  exit 1
fi

IMG=m6-alpine
NAME=m6-web
NET=m6-net
"$TINY" image import --name "$IMG" --file "$ALPINE_TGZ" >/dev/null
"$TINY" network create --name "$NET" --cidr 10.70.0.0/24 >/dev/null
"$TINY" instance create --name "$NAME" --image "$IMG" --cpu 1 --memory 64MB --network "$NET" >/dev/null

printf '== metadata JSON ==\n'
META="$("$TINY" instance metadata "$NAME")"
printf '%s\n' "$META"
printf '%s' "$META" | python3 -c 'import sys,json; d=json.load(sys.stdin); assert d["name"]=="m6-web" and d["local-ipv4"]=="10.70.0.2" and d["veth-host"], d'
VETH="$(printf '%s' "$META" | python3 -c 'import sys,json; print(json.load(sys.stdin)["veth-host"])')"

printf '== meta-data path ==\n'
ID="$("$TINY" instance metadata "$NAME" instance-id)"
printf '%s\n' "$ID"

printf '== by-veth ==\n'
curl -sf -H "Authorization: Bearer ${TINY_TOKEN}" \
  "http://${TINY_API_ADDR}/v1/metadata/by-veth/${VETH}" | python3 -c 'import sys,json; d=json.load(sys.stdin); assert d["name"]=="m6-web", d'

printf '== stop → metadata conflict ==\n'
"$TINY" instance stop "$NAME" >/dev/null
if "$TINY" instance metadata "$NAME" >/dev/null 2>&1; then
  printf 'expected conflict when stopped\n' >&2
  exit 1
fi

"$TINY" instance delete "$NAME" >/dev/null
"$TINY" network delete "$NET" >/dev/null
"$TINY" image delete "$IMG" >/dev/null

printf 'M6 smoke passed\n'
