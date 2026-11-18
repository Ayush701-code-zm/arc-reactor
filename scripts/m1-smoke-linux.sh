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

export TINY_DB="${TINY_DB:-$ROOT/data/tiny-docker.db}"
export TINY_RUNTIME_ADDR="${TINY_RUNTIME_ADDR:-127.0.0.1:50051}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"

printf '== start tinyruntimed (TINY_REQUIRE_CGROUP=%s) ==\n' "$TINY_REQUIRE_CGROUP"
"$BUILD_DIR/tinyruntimed" --addr 127.0.0.1:50051 >/tmp/tinyruntimed.log 2>&1 &
RPID=$!
cleanup() {
  if kill -0 "$RPID" 2>/dev/null; then
    kill "$RPID" 2>/dev/null || true
    wait "$RPID" 2>/dev/null || true
  fi
}
trap cleanup EXIT
sleep 0.5

TINY=control-plane/bin/tiny
NAME="smoke-$(date +%s)"

printf '== create ==\n'
CREATE_JSON="$("$TINY" instance create --name "$NAME" --image alpine --cpu 1 --memory 64MB)"
printf '%s\n' "$CREATE_JSON"
ID="$(printf '%s' "$CREATE_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["ID"])')"

if [[ "$TINY_REQUIRE_CGROUP" == "1" ]]; then
  printf '== assert cgroup limits ==\n'
  PARENT="${TINY_CGROUP_PARENT:-/sys/fs/cgroup}"
  CGDIR="${PARENT}/tinyaws/${ID}"
  if [[ ! -f "${CGDIR}/cpu.max" || ! -f "${CGDIR}/memory.max" ]]; then
    printf 'missing cpu.max/memory.max under %s\n' "$CGDIR" >&2
    exit 1
  fi
  printf 'ok %s/{cpu.max,memory.max}\n' "$CGDIR"
fi

printf '== list ==\n'
"$TINY" instance list

printf '== inspect ==\n'
"$TINY" instance inspect "$NAME"

printf '== stop ==\n'
"$TINY" instance stop "$NAME"

printf '== start ==\n'
"$TINY" instance start "$NAME"

printf '== delete ==\n'
"$TINY" instance delete "$NAME"

printf 'M1 smoke passed for %s\n' "$NAME"
if [[ "$TINY_REQUIRE_CGROUP" != "1" ]]; then
  printf 'Note: cgroup limits may be skipped on Docker Desktop. Use TINY_REQUIRE_CGROUP=1 on real Ubuntu/CI.\n'
fi
