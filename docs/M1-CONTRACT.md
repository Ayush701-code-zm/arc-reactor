# Milestone 1 — locked create / lifecycle contract

Decisions locked with the user. Do not change without an explicit new decision.

Source proto: [`proto/tiny/v1/runtime.proto`](../proto/tiny/v1/runtime.proto)

## Transport note (interim)

Field names and RPCs are defined by the `.proto` file.
Until `scripts/gen-proto.sh` produces gRPC stubs, `tinyruntimed` speaks an HTTP/JSON mirror of those fields on `127.0.0.1:50051`.
Replace with gRPC without changing the field contract.

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

1. Send `SIGTERM`
2. Wait **5000 ms**
3. If still alive, send `SIGKILL`

While waiting, status is `STOPPING`. When done, status is `STOPPED`. Timeout is fixed for M1; configurable later.

## Create order and compensation

Order: **DB-first**, then gRPC `CreateInstance`.

| Failure | Compensate |
|---|---|
| SQLite insert fails before gRPC | No runtime call. Return error. No orphan. |
| SQLite ok, gRPC / startup fails | Mark row `FAILED` (or delete per reconcile policy) using `error_message`. No pretend-running row. |
| gRPC reports running, later DB update fails | Best-effort runtime `DeleteInstance`, then reconcile so no orphan process and no ghost row. |

Go periodically reconciles desired SQLite state with the runtime live view.

## Status values

`PENDING`, `RUNNING`, `STOPPING`, `STOPPED`, `FAILED`
