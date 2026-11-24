---
description: Repo map, folder boundaries, roadmap phase gate, and Milestone 1 scope. Always apply.
alwaysApply: true
trigger: always_on
---

# Repo map

Tiny AWS is a from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

**v1 is done** (Phases 0–8). Post-v1 versions are phased in [`docs/VERSIONS.md`](../../docs/VERSIONS.md). Phase history: [`docs/ROADMAP.md`](../../docs/ROADMAP.md). Do not start v2+ until the user explicitly opens that version, then one phase inside it.

| Path | Contents |
|---|---|
| `control-plane/` | Go module. `cmd/tiny`, `cmd/tinyd`, `internal/` |
| `runtime/` | C++20 CMake project. `include/`, `src/`, `tests/` |
| `observability/tinyebpf/` | C tinyebpf daemon (BPF + UDS) |
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
| `rules/always/response-style.md` | Crisp English replies, repo language, hop-by-hop change flowcharts |
| `rules/always/safety.md` | Ask before DB writes, process kill, protected files |
| `rules/always/simple-answers.md` | Answer first, short plans, who acts |
| `rules/always/thinking-questions.md` | Socratic build mode before non-trivial work |
| `rules/always/vortex-verification.md` | Early verification in the coding loop |
| `rules/always/code-comments.md` | No AI-style narrating comments in source |

# Working rules

- Edit `rules/always/` and `rules/scoped/`, then run `scripts/sync-agent-rules.sh`. Do not edit `AGENTS.md` by hand.
- v1 (Phases 0–8) remains done and supported. See `docs/ROADMAP.md` and `docs/M8-CONTRACT.md`.
- Post-v1 work follows `docs/VERSIONS.md`. Open a **version** explicitly, then open **one phase** inside it. Do not jump to the next phase or version early.
- If the user asks for a later-version feature early, remind them of the version and phase gate and ask whether to stay on v1 polish or explicitly open that version and its first locked phase.
- Go changes stay in `control-plane/`. C++ changes stay in `runtime/`. eBPF C stays in `observability/`. A contract change starts in `proto/tiny/v1/`.
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

# Version lock (current)

v1 complete. Default: v1 polish only (bugs, docs, smoke clarity) unless the user opens a version.

Next planned open: **v2**, phase **V2.1 Image on node** only. Later v2 phases are V2.2 agent-owned ops, then V2.3 two-node proof. See `docs/VERSIONS.md`.

Out of v2 unless the contract says otherwise: overlay/VXLAN product, fancy IAM, snapshots, HA tinyd.
