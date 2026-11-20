---
description: Go control plane boundaries for Tiny AWS
alwaysApply: false
trigger: glob
globs:
  - "control-plane/**"
  - "**/*.go"
paths:
  - "control-plane/**"
  - "**/*.go"
---

# Go control plane

Locked M1 contract: [`docs/M1-CONTRACT.md`](../../docs/M1-CONTRACT.md).
Locked M3 volume contract: [`docs/M3-CONTRACT.md`](../../docs/M3-CONTRACT.md).

- `cmd/tiny` is the CLI. `cmd/tinyd` is the daemon. Shared behavior lives in `internal/`.
- Go assigns the UUID id, enforces unique `--name`, and owns desired state in SQLite.
- CLI takes names. Resolve name → id before runtime RPCs.
- Validate name, image, CPU, and memory in Go before any RPC.
- CPU is a whole number of 1 or more. Memory is bytes on the RPC. The CLI accepts sizes such as `512MB`.
- Create order: DB-first, then gRPC `CreateInstance`. Compensate and reconcile on failure.
- Do not fork, set namespaces, or write cgroup files in Go.
- Do not mark an instance running unless the runtime reported that state.
- Do not send `command` / `args` in M1. Image implies a fixed stub on the runtime.
- Return not implemented until the store and the runtime client exist. Do not invent a success response.
- SQLite writes are dev state. Ask the user before a change that inserts, updates, or deletes rows.
- Tests live next to the package they cover. `go test ./...` must pass.

# CLI

| Command | Required arguments |
|---|---|
| `tiny instance create` | `--name`, `--image`, `--cpu`, `--memory` |
| `tiny instance list` | none |
| `tiny instance inspect` | name |
| `tiny instance stop` | name |
| `tiny instance start` | name |
| `tiny instance delete` | name |
| `tiny volume create` | `--name`, `--size` |
| `tiny volume list` | none |
| `tiny volume inspect` | name |
| `tiny volume delete` | name |
| `tiny volume attach` | `--volume`, `--instance` |
| `tiny volume detach` | `--volume` |
