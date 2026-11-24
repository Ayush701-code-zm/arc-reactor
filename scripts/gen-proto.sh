#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
missing=0

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing %s\n' "$1" >&2
    missing=1
  fi
}

need protoc
need protoc-gen-go
need protoc-gen-go-grpc

if [[ "$missing" -ne 0 ]]; then
  printf 'Install protoc, protoc-gen-go, and protoc-gen-go-grpc.\n' >&2
  exit 1
fi

mkdir -p "$ROOT/control-plane"
(
  cd "$ROOT"
  protoc -I proto \
    --go_out=control-plane \
    --go_opt=module=github.com/ayushanand/tiny-aws/control-plane \
    --go-grpc_out=control-plane \
    --go-grpc_opt=module=github.com/ayushanand/tiny-aws/control-plane \
    proto/tiny/v1/runtime.proto \
    proto/tiny/v1/node.proto
)

printf 'Wrote control-plane/gen from proto/tiny/v1/*.proto\n'
