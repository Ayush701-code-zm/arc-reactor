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

# Force dedicated ports/roots. Interactive shells often export TINY_API_ADDR=:8080
# which would make ${VAR:-default} keep the old daemon and break import.
export TINY_DB="${TINY_M3_DB:-$ROOT/data/tiny-m3.db}"
export TINY_RUNTIME_ADDR="${TINY_M3_RUNTIME_ADDR:-127.0.0.1:15051}"
export TINY_API_ADDR="${TINY_M3_API_ADDR:-127.0.0.1:18080}"
export TINY_IMAGE_ROOT="${TINY_M3_IMAGE_ROOT:-$ROOT/data/images-m3}"
export TINY_VOLUME_ROOT="${TINY_M3_VOLUME_ROOT:-$ROOT/data/volumes-m3}"
export TINY_REQUIRE_CGROUP="${TINY_REQUIRE_CGROUP:-0}"
mkdir -p "$TINY_IMAGE_ROOT" "$TINY_VOLUME_ROOT"
rm -f "$TINY_DB"
printf 'M3 endpoints: api=%s runtime=%s db=%s\n' "$TINY_API_ADDR" "$TINY_RUNTIME_ADDR" "$TINY_DB"

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

printf '== start tinyruntimed on %s (TINY_REQUIRE_CGROUP=%s) ==\n' "$TINY_RUNTIME_ADDR" "$TINY_REQUIRE_CGROUP"
"$BUILD_DIR/tinyruntimed" --addr "$TINY_RUNTIME_ADDR" >/tmp/tinyruntimed-m3.log 2>&1 &
RPID=$!

printf '== start tinyd (http://%s) ==\n' "$TINY_API_ADDR"
control-plane/bin/tinyd >/tmp/tinyd-m3.log 2>&1 &
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
    cat /tmp/tinyd-m3.log >&2 || true
    cat /tmp/tinyruntimed-m3.log >&2 || true
    exit 1
  fi
  if curl -sf "http://${TINY_API_ADDR}/healthz" >/dev/null; then
    break
  fi
  if [[ "$i" -eq 10 ]]; then
    printf 'tinyd did not become ready on %s\n' "$TINY_API_ADDR" >&2
    cat /tmp/tinyd-m3.log >&2 || true
    cat /tmp/tinyruntimed-m3.log >&2 || true
    exit 1
  fi
  sleep 0.3
done

TINY=control-plane/bin/tiny
IMG="m3-alpine"
NAME="m3-inst-$(date +%s)"
VOL="m3-data"

printf '== image import ==\n'
if [[ ! -f "$ALPINE_TGZ" ]]; then
  printf 'missing alpine archive %s\n' "$ALPINE_TGZ" >&2
  exit 1
fi
"$TINY" image import --name "$IMG" --file "$ALPINE_TGZ"

printf '== instance create ==\n'
CREATE_JSON="$("$TINY" instance create --name "$NAME" --image "$IMG" --cpu 1 --memory 64MB)"
printf '%s\n' "$CREATE_JSON"
ROOTFS="$(printf '%s' "$CREATE_JSON" | python3 -c 'import sys,json; print(json.load(sys.stdin)["RootfsPath"])')"
if [[ -z "$ROOTFS" ]]; then
  printf 'RootfsPath empty on create\n' >&2
  exit 1
fi

printf '== instance list has RootfsPath ==\n'
LIST_JSON="$("$TINY" instance list)"
printf '%s\n' "$LIST_JSON"
printf '%s' "$LIST_JSON" | python3 -c 'import sys,json; xs=json.load(sys.stdin); assert xs and xs[0].get("RootfsPath"), xs'

printf '== volume create ==\n'
"$TINY" volume create --name "$VOL" --size 64MB

printf '== volume attach ==\n'
ATTACH_JSON="$("$TINY" volume attach --volume "$VOL" --instance "$NAME")"
printf '%s\n' "$ATTACH_JSON"
printf '%s' "$ATTACH_JSON" | python3 -c 'import sys,json; v=json.load(sys.stdin); assert v["Status"]=="ATTACHED" and v.get("DevicePath"), v'

MNT="$ROOTFS/mnt/$VOL"
printf '== assert mount dir %s ==\n' "$MNT"
if [[ ! -d "$MNT" ]]; then
  printf 'missing mount dir\n' >&2
  cat /tmp/tinyruntimed-m3.log >&2 || true
  exit 1
fi

printf '== volume detach ==\n'
"$TINY" volume detach --volume "$VOL"

printf '== volume re-attach then instance delete (auto-detach) ==\n'
"$TINY" volume attach --volume "$VOL" --instance "$NAME"
"$TINY" instance delete "$NAME"
DET_JSON="$("$TINY" volume inspect "$VOL")"
printf '%s\n' "$DET_JSON"
printf '%s' "$DET_JSON" | python3 -c 'import sys,json; v=json.load(sys.stdin); assert v["Status"]=="AVAILABLE", v'

printf '== volume delete ==\n'
"$TINY" volume delete "$VOL"

printf '== image delete ==\n'
"$TINY" image delete "$IMG"

printf 'M3 smoke passed for volume=%s instance=%s\n' "$VOL" "$NAME"
