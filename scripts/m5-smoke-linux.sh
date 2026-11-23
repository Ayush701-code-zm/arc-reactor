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

export TINY_DB="${TINY_M5_DB:-$ROOT/data/tiny-m5.db}"
export TINY_RUNTIME_ADDR="${TINY_M5_RUNTIME_ADDR:-127.0.0.1:15053}"
export TINY_API_ADDR="${TINY_M5_API_ADDR:-127.0.0.1:18082}"
export TINY_IMAGE_ROOT="${TINY_M5_IMAGE_ROOT:-$ROOT/data/images-m5}"
export TINY_VOLUME_ROOT="${TINY_M5_VOLUME_ROOT:-$ROOT/data/volumes-m5}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"
mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT"
rm -f "$TINY_DB"

"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-m5.log 2>&1 &
RPID=$!
control-plane/bin/tinyd >/tmp/tinyd-m5.log 2>&1 &
DPID=$!
cleanup() {
  kill "$DPID" "$RPID" 2>/dev/null || true
  wait "$DPID" "$RPID" 2>/dev/null || true
}
trap cleanup EXIT

for i in 1 2 3 4 5 6 7 8 9 10; do
  curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null && break
  sleep 0.3
done

TINY=control-plane/bin/tiny

printf '== bootstrap ==\n'
"$TINY" auth bootstrap --username admin --password adminadmin

printf '== second bootstrap must conflict ==\n'
if "$TINY" auth bootstrap --username other --password adminadmin 2>/tmp/boot2.err; then
  printf 'expected conflict\n' >&2
  exit 1
fi

printf '== login ==\n'
LOGIN_JSON="$("$TINY" auth login --username admin --password adminadmin)"
export TINY_TOKEN="$(printf '%s' "$LOGIN_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["AccessToken"])')"
REFRESH="$(printf '%s' "$LOGIN_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["RefreshToken"])')"

printf '== whoami ==\n'
"$TINY" auth whoami | python3 -c 'import sys,json; u=json.load(sys.stdin); assert u["Username"]=="admin" and u["Role"]=="admin", u'

printf '== unauthenticated list must fail ==\n'
if env -u TINY_TOKEN "$TINY" instance list >/dev/null 2>&1; then
  printf 'expected 401 without token\n' >&2
  exit 1
fi

printf '== refresh ==\n'
REF_JSON="$("$TINY" auth refresh --refresh-token "$REFRESH")"
export TINY_TOKEN="$(printf '%s' "$REF_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["AccessToken"])')"

printf '== create owned network ==\n'
"$TINY" network create --name m5-net --cidr 10.60.0.0/24 >/dev/null
"$TINY" network list | python3 -c 'import sys,json; xs=json.load(sys.stdin); assert len(xs)==1 and xs[0]["OwnerID"], xs'

printf 'M5 smoke passed\n'
