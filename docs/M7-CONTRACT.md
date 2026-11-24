# Milestone 7 — locked observability (eBPF) contract

Phase 7 is open. Phases 1–6 stay complete.

## Phase status

| Phase | Status |
|---|---|
| 1–6 | done |
| 7 eBPF observability | done |
| 8 Multi-node | CURRENT |

## Ownership

| Piece | Owner |
|---|---|
| BPF programs + map poll + cgroup cpu/mem file samples | **`tinyebpf` (C)** |
| UDS client, inspect enrichment, create/delete watch | **Go tinyd** |
| C++ runtime | unchanged (still creates cgroup v2); no gRPC metrics stream in M7 |

## Signals (M7)

| Signal | Source |
|---|---|
| CPU / memory | cgroup v2 files (`cpu.stat`, `memory.current`) via tinyebpf |
| Packet / byte counters | `cgroup/skb` ingress + egress maps, polled by tinyebpf |

## Transport

| Piece | Rule |
|---|---|
| Process | dedicated `tinyebpf` |
| IPC | Unix domain socket (default `$TINY_EBPF_SOCK` or `/tmp/tiny-aws-ebpf.sock`) |
| Control plane | tinyd requests `watch` / `unwatch` / `sample` as JSON lines |
| CLI surface | fields on `tiny instance inspect` |

## Attach

| Piece | Rule |
|---|---|
| Program type | `cgroup/skb` ingress and egress |
| Target | per-instance cgroup v2 directory (resolved from instance PID `/proc/<pid>/cgroup`) |
| Default | eBPF optional; create stays RUNNING if attach fails (`ebpf=skipped`) |
| Strict | `TINY_REQUIRE_EBPF=1` → create fails if watch/sample readiness fails |

## Out of Phase 7

XDP firewall, full continuous profiler, multi-node metric fan-in, ring-buffer → C++ gRPC → Go pipeline.
