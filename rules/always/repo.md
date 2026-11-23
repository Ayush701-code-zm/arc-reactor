---
description: Repo map, folder boundaries, roadmap phase gate, and Milestone 1 scope. Always apply.
alwaysApply: true
trigger: always_on
---

# Repo map

Tiny AWS is a from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

Current phase: **Phase 6 — Metadata**. Phases 1–5 are done. Source of truth: [`docs/ROADMAP.md`](../../docs/ROADMAP.md).

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
- Stay on Phase 6 until its exit criteria are met. Do not jump to Phase 7+ early. See `docs/ROADMAP.md` and `docs/M6-CONTRACT.md`.
- Phases 1–5 remain done and supported; do not rip out instance/image/volume/network/IAM while building metadata.
- If the user asks for a later-phase feature early, remind them of the roadmap gate and ask whether to stay on Phase 6 or explicitly open that phase.
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

# Phase 6 lock (current)

In scope: tinyd metadata JSON + AWS-style paths, owner Bearer access, RUNNING-only, network fields from applied attachments, bind via host veth (`/v1/metadata/by-veth/{veth}`).

Out of Phase 6: in-guest 169.254.169.254, C++ DNAT/forward, IMDSv2, user-data, eBPF, multi-node.

Do not jump to Phase 7+ unless the user explicitly opens that phase.
