# Milestone 1 — Tiny EC2 (CURRENT)

Working Linux instance manager on one Ubuntu machine.

**Do not leave Milestone 1 early.** Phase 2+ stays locked until M1 exit criteria are met, unless the user explicitly opens the next phase. Full phase list: [`docs/ROADMAP.md`](ROADMAP.md).

## Goal

A demoable instance lifecycle:

```bash
tiny instance create \
  --name my-server \
  --image alpine \
  --cpu 1 \
  --memory 512MB
```

## Required flow

1. Receive request through Go CLI
2. Send it to Go control plane
3. Validate instance configuration
4. Ask C++ runtime to create the instance
5. Prepare an isolated Linux environment
6. Apply CPU and memory limits
7. Start and supervise the process
8. Return instance ID and status

## Commands

| Command | Meaning |
|---|---|
| `tiny instance create` | Validate, store state, ask the runtime to start |
| `tiny instance list` | Show instances |
| `tiny instance inspect <name>` | Show one instance |
| `tiny instance stop <name>` | Stop a running instance |
| `tiny instance start <name>` | Start a stopped instance |
| `tiny instance delete <name>` | Remove an instance |

## Locked decisions

Full contract: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md) and [`proto/tiny/v1/runtime.proto`](../proto/tiny/v1/runtime.proto).

| Topic | Lock |
|---|---|
| Identity | UUID primary + unique `--name` alias |
| Ownership | Go: id + desired state; C++: live process; Go reconciles |
| Create request | `id`, `name`, `image`, `cpu`, `memory_bytes` (no command/args) |
| Create response | `id`, `status`, `pid`, `error_message` (startup failures); gRPC codes for RPC failures |
| Isolation | pid namespace only in M1 |
| Limits | cgroup v2 `cpu.max` + `memory.max` |
| Stop | SIGTERM → 5000 ms → SIGKILL; status `STOPPING` then `STOPPED` |
| Create order | DB-first, then gRPC, with compensation + reconciliation |

## Includes

- Go CLI + control plane validation/orchestration
- SQLite metadata/state
- gRPC to C++ runtime
- pid namespace + cgroup v2 CPU/memory limits
- process start / supervise / stop / start / delete

## Excludes (do not build in M1)

- Full virtual networking product
- Block storage product
- IAM product depth
- eBPF
- Multi-node scheduling
- Other AWS-like services beyond instance lifecycle
- Docker / Kubernetes / AWS API wrappers / Rust

## Exit criteria

- All M1 commands work reliably on one Ubuntu machine
- Instance is actually isolated and CPU/memory limited
- No orphan processes or ghost DB rows on failure paths
- Demoable live with `tiny instance *`

## Agent workflow

- Before non-trivial builds, ask design and technical questions that lead to the solution. Do not jump straight to code.
- Use early verification while coding (Vortex-style): cheapest local checks after each meaningful change. SonarQube is the analysis source when present. Vortex makes it available earlier. Compiler and tests are enough until that is wired.
- Lock the decision in a few bullets, then implement only that decision.
- If asked for a later-phase feature, cite the roadmap gate and ask whether to stay on M1 or open that phase.
