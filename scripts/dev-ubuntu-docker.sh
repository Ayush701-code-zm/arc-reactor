#!/bin/bash
# Opens an Ubuntu 24.04 container as the Linux *host* for Tiny AWS.
# Docker is NOT the instance runtime. Instances still use pid ns + cgroup v2.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
IMAGE="${TINY_DEV_IMAGE:-tiny-aws-dev:24.04}"
NAME="${TINY_DEV_NAME:-tiny-aws-dev}"

if ! command -v docker >/dev/null 2>&1; then
  printf 'docker not found. Install Docker Desktop, then retry.\n' >&2
  exit 1
fi

arch="$(uname -m)"
go_arch="amd64"
case "$arch" in
  arm64|aarch64) go_arch="arm64" ;;
esac

printf 'Building %s (Go linux-%s)...\n' "$IMAGE" "$go_arch"
docker build \
  --build-arg "GO_VERSION=${GO_VERSION:-1.22.12}" \
  --build-arg "GO_ARCH=${go_arch}" \
  -f "$ROOT/Dockerfile.dev" \
  -t "$IMAGE" \
  "$ROOT"

# Private cgroup namespace (default) + privileged lets the container treat
# /sys/fs/cgroup as its own root and enable +cpu +memory for tinyaws/.
# Host cgroupns made Docker Desktop leave cpu/memory undelegated under /docker/<id>.
printf 'Starting privileged Ubuntu host container (private cgroupns).\n'
printf 'Inside: bash scripts/m1-smoke-linux.sh\n'
printf 'If a container named %s is already running, exit it first.\n' "$NAME"

exec docker run --rm -it \
  --name "$NAME" \
  --privileged \
  --cgroupns=private \
  -v "$ROOT":/src \
  -w /src \
  -e TINY_DB=/src/data/tiny-docker.db \
  -e TINY_RUNTIME_ADDR=127.0.0.1:50051 \
  "$IMAGE" \
  bash
