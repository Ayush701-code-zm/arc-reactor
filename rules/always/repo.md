---
description: Repo map, folder boundaries, roadmap phase gate, and Milestone 1 scope. Always apply.
alwaysApply: true
trigger: always_on
---

# Repo map

Tiny AWS is a from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

Current phase: **Phase 3 Storage is done.** Phase 4+ is locked until explicitly opened. Source of truth: [`docs/ROADMAP.md`](../../docs/ROADMAP.md).

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
| `rules/always/response-style.md` | Crisp English replies, repo language, hop-by-hop change flowcharts |
| `rules/always/safety.md` | Ask before DB writes, process kill, protected files |
| `rules/always/simple-answers.md` | Answer first, short plans, who acts |
| `rules/always/thinking-questions.md` | Socratic build mode before non-trivial work |
| `rules/always/vortex-verification.md` | Early verification in the coding loop |
| `rules/always/code-comments.md` | No AI-style narrating comments in source |

# Working rules

- Edit `rules/always/` and `rules/scoped/`, then run `scripts/sync-agent-rules.sh`. Do not edit `AGENTS.md` by hand.
- Phase 3 exit criteria are met. Do not jump to Phase 4+ unless the user explicitly opens it. See `docs/ROADMAP.md` and `docs/M3-CONTRACT.md`.
- Phase 1–3 remain done and supported; do not rip out instance, image, or volume lifecycle.
- If the user asks for a later-phase feature early, remind them of the roadmap gate and ask whether to stay post-Phase-3 or explicitly open that phase.
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

# Phase 3 complete (supported)

Done: volume create/list/inspect/delete, sparse `disk.img`, attach/detach loop+mount at `/mnt/<name>`, detach-on-instance-delete, `RootfsPath` in SQLite, `scripts/m3-smoke-linux.sh`.

Still out until a later phase opens: snapshots, multi-attach, remote block store, encryption, online resize, networking product, IAM depth, metadata, eBPF, multi-node.

Do not jump to Phase 4+ unless the user explicitly opens that phase.
