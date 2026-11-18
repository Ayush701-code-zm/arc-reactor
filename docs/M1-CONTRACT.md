# Milestone 1 — locked create / lifecycle contract

Decisions locked with the user. Do not change without an explicit new decision.

Source proto: [`proto/tiny/v1/runtime.proto`](../proto/tiny/v1/runtime.proto)

## Transport

Field names and RPCs are defined by `proto/tiny/v1/runtime.proto`.
M1 wire protocol is **gRPC only** on `127.0.0.1:50051` (override with `TINY_RUNTIME_ADDR`).
Go stubs live in `control-plane/gen/` (committed). C++ stubs are generated at CMake build time.

## Identity

| Piece | Rule |
|---|---|
| Primary id | UUID assigned by the Go control plane |
| Human handle | `--name`, unique alias |
| CLI | Users pass name; Go resolves name → id before runtime RPCs |

## Ownership

| Owner | Owns |
|---|---|
| Go | Identity, desired state, SQLite metadata, reconciliation |
| C++ | Live process table, pid, namespaces/cgroups, actual running/stopped |

## Create RPC

**Request (required):** `id`, `name`, `image`, `cpu`, `memory_bytes`

**Not in M1:** `command`, `args`. The image runs a fixed stub command. Command overrides are a later phase.

**Response:** `id`, `status`, `pid`, `error_message`

| Failure kind | How it is reported |
|---|---|
| Transport / RPC failure | gRPC status code |
| Instance startup failure | Response with `status = FAILED` and `error_message` set |

## Isolation and limits (M1)

| Mechanism | M1 choice |
|---|---|
| Namespaces | **pid** only (isolate process tree) |
| Mount / uts / net / user | Later milestones |
| CPU / memory | cgroup v2: `cpu.max`, `memory.max` under `<self-cgroup>/tinyaws/<id>/` |

## Stop semantics

1. C++ sends `SIGTERM`
2. Wait **5000 ms**
3. If still alive, C++ sends `SIGKILL` and waits for exit

While waiting, status is `STOPPING`. Go persists `STOPPED` **only after** C++ confirms the process exited (`status = STOPPED` in the Stop response). Timeout is fixed for M1; configurable later.

## Create order and compensation

Order: **DB-first**, then gRPC `CreateInstance`.

| Failure | Compensate |
|---|---|
| SQLite insert fails before gRPC | No runtime call. Return error. No orphan. |
| SQLite ok, gRPC / startup fails | Mark row `FAILED` using `error_message`. No pretend-running row. |
| gRPC reports running, later DB update fails | Go owns cleanup: call runtime `DeleteInstance` (C++ terminates the process), then remove the SQLite row. If runtime cleanup fails, keep the row as `FAILED` with both errors. |

## Delete semantics

1. Go calls runtime `DeleteInstance` (C++ stops if needed, then removes runtime state / cgroup).
2. If the runtime cannot confirm termination, **fail delete** and **keep the SQLite row**.
3. Only after runtime confirm, Go deletes the SQLite row.

## Cgroup smoke assert (when `TINY_REQUIRE_CGROUP=1`)

After create, both files must exist under the instance cgroup directory:

- `cpu.max`
- `memory.max`

## Status values

`PENDING`, `RUNNING`, `STOPPING`, `STOPPED`, `FAILED`
