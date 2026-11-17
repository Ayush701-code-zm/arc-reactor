#!/bin/bash
# Generates Go and C++ stubs from proto/tiny/v1/runtime.proto.
# Does not run as part of the foundation build. Run it when the plugins exist.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PROTO="$ROOT/proto/tiny/v1/runtime.proto"
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
need grpc_cpp_plugin

if [[ "$missing" -ne 0 ]]; then
  printf 'Install protoc, protoc-gen-go, protoc-gen-go-grpc, and grpc_cpp_plugin, then run this script again.\n' >&2
  exit 1
fi

mkdir -p "$ROOT/control-plane" "$ROOT/runtime/gen"
(
  cd "$ROOT"
  protoc -I proto \
    --go_out=control-plane \
    --go_opt=module=github.com/ayushanand/tiny-aws/control-plane \
    --go-grpc_out=control-plane \
    --go-grpc_opt=module=github.com/ayushanand/tiny-aws/control-plane \
    proto/tiny/v1/runtime.proto
  protoc -I proto \
    --cpp_out=runtime/gen \
    --grpc_out=runtime/gen \
    --plugin=protoc-gen-grpc="$(command -v grpc_cpp_plugin)" \
    proto/tiny/v1/runtime.proto
)

printf 'Generated stubs from %s\n' "$PROTO"
printf 'Generated files are not the source of truth. Edit the .proto file, then run this script again.\n'
