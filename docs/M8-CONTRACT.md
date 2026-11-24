# Milestone 8 — locked multi-node contract

Phase 8 is open. Phases 1–7 stay complete. Do not change these locks without an explicit new decision.

## Phase status

| Phase | Status |
|---|---|
| 1–7 | done |
| 8 Multi-node | done (v1 complete) |
| Post-v1 versions | [`docs/VERSIONS.md`](VERSIONS.md); locked until opened |

## Ownership

| Piece | Owner |
|---|---|
| Scheduler (first-fit), node registry, allocated capacity in SQLite, create orchestration | **Go tinyd** (`internal/scheduler`, `internal/nodehub`) |
| Node agent binary, host capacity sample, proxy create/stop/start/delete/inspect to local runtime | **C++ `tinynodeagent`** |
| Instance process / cgroup / netns | **C++ `tinyruntimed`** (unchanged; agent is a client of it) |

## Create hop (locked option A)

```
tiny → tinyd HTTP
  → scheduler first-fit → node_id
  → SQLite insert (with node_id)
  → NodeControlSession gRPC (agent already dialed in)
  → tinynodeagent
  → local RuntimeService gRPC → tinyruntimed
```

tinyd does **not** open a second runtime dial for create. Volume/network attach may still use the single `TINY_RUNTIME_ADDR` dial (same-node assumption in M8 smoke).

## Registration

| Piece | Rule |
|---|---|
| Direction | Agent dials control plane (`TINY_NODE_CONTROL_ADDR`, default `127.0.0.1:50052`) |
| Identity | Agent-stable UUID (`TINY_NODE_ID` or file); hostname informational |
| Transport | Bidirectional gRPC stream `NodeControlService.Session` |
| Heartbeat | Agent sends allocatable CPU/mem + local running count |

## Capacity (v1)

| Signal | Role |
|---|---|
| Allocatable CPU cores + memory bytes | From agent heartbeat (host-level) |
| Allocated CPU + memory | Summed by tinyd from SQLite instances on that `node_id` (PENDING/RUNNING/STOPPING) |
| Running instance count | Informational only (not a placement constraint) |
| Disk | Not used for placement |

## Scheduler

| Rule | M8 behavior |
|---|---|
| Process | In-process Go package used by tinyd (scheduler service) |
| Algorithm | First-fit over connected nodes with free CPU and free memory |
| Full cluster | Hard-fail create (`no node with capacity`) |
| Queue / image affinity | Out of M8 |

## Instance row

Every instance stores `node_id`. Inspect JSON includes `NodeID`.

## Legacy path

When **zero** agents are connected, create may fall back to the single `TINY_RUNTIME_ADDR` dial and set `node_id=local` so Phases 1–7 smokes keep working. When ≥1 agent is connected, create **must** go through the agent path (no silent local bypass).

## Out of Phase 8

Cross-node overlay / VXLAN, shared storage across nodes, live migration, tinyd HA / etcd, multi-dial of `tinyruntimed` from tinyd, queuing, image affinity, disk-aware scheduling.
