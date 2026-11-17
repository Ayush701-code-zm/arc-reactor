---
description: Repo map, folder boundaries, roadmap phase gate, and Milestone 1 scope. Always apply.
alwaysApply: true
trigger: always_on
---

# Repo map

Tiny AWS is a from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

Current phase: **Phase 1 — Milestone 1 Tiny EC2**. Source of truth for now vs later vs end: [`docs/ROADMAP.md`](../../docs/ROADMAP.md).

| Path | Contents |
|---|---|
| `control-plane/` | Go module. `cmd/tiny`, `cmd/tinyd`, `internal/` |
| `runtime/` | C++20 CMake project. `include/`, `src/`, `tests/` |
| `proto/tiny/v1/` | gRPC contract between Go and C++ |
| `rules/always/` | Always-on rules. Source of truth |
| `rules/scoped/` | Path-scoped rules. Not copied into `AGENTS.md` |
| `.cursor/rules/` | Symlinks to the rules. Do not edit |
| `hooks/` | Safety hook for agent shell commands |
| `docs/ROADMAP.md` | Phase checklist and destination |
| `docs/ARCHITECTURE.md` | Language ownership and system shape |
| `docs/MILESTONE-1.md` | Current focus detail |
| `scripts/sync-agent-rules.sh` | Rebuilds `AGENTS.md` and refreshes Cursor symlinks |

# Always-on rules

| File | What it locks |
|---|---|
| `rules/always/repo.md` | Repo map, roadmap pointer, Milestone 1 scope |
| `rules/always/architecture.md` | Ownership, hard bans, phase gate |
| `rules/always/response-style.md` | Crisp English replies and repo language |
| `rules/always/safety.md` | Ask before DB writes, process kill, protected files |
| `rules/always/simple-answers.md` | Answer first, short plans, who acts |
| `rules/always/thinking-questions.md` | Socratic build mode before non-trivial work |
| `rules/always/vortex-verification.md` | Early verification in the coding loop |

# Working rules

- Edit `rules/always/` and `rules/scoped/`, then run `scripts/sync-agent-rules.sh`. Do not edit `AGENTS.md` by hand.
- Stay on Phase 1 until its exit criteria are met. Do not leave Milestone 1 early. See `docs/ROADMAP.md` and `docs/MILESTONE-1.md`.
- Later phases are documented so we do not forget the destination. Do not "helpfully" implement them during M1.
- If the user asks for a later-phase feature early, remind them of the roadmap gate and ask whether to stay on M1 or explicitly open that phase.
- Go changes stay in `control-plane/`. C++ changes stay in `runtime/`. A contract change starts in `proto/tiny/v1/`.
- Keep the tree compilable. A stub with a TODO is the right placeholder.
- Build and test with the commands in `README.md`.
- Before non-trivial builds, ask design and technical questions. See `thinking-questions.md`.
- After meaningful edits, run early verification. See `vortex-verification.md`.

Shared contracts, loaded when their paths are open:

| File | What it locks |
|---|---|
| `rules/scoped/go-control-plane.md` | Go CLI, REST, SQLite, orchestration |
| `rules/scoped/cpp-runtime.md` | C++20 runtime, processes, limits |
| `rules/scoped/proto-contract.md` | `proto/tiny/v1` gRPC contract |

# Milestone 1 lock (current)

In scope: `create`, `list`, `inspect`, `stop`, `start`, and `delete` on one Linux machine, with isolation and CPU/memory limits as decided for M1.

Out of Milestone 1: images product, storage product, full VPC, IAM depth, metadata service, eBPF, multi-node, Docker, Kubernetes, AWS API wrappers, Rust.

Do not jump to Phase 2+ unless the user explicitly opens that phase.
