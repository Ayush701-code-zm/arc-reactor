# Milestone 1 — Tiny EC2 (done, historical)

Working Linux instance manager on one Ubuntu machine. **Part of completed v1** (Phases 0–8).

Do not treat this file as the current focus. Current plan: [`docs/VERSIONS.md`](VERSIONS.md). Phase history: [`docs/ROADMAP.md`](ROADMAP.md). Locked create contract: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md).

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

## Ownership (M1)

| Piece | Owner |
|---|---|
| CLI, REST, SQLite, orchestration | Go |
| Process, namespaces, cgroups | C++ |
| Wire contract | `proto/tiny/v1/runtime.proto` |

## Exit criteria (met in v1)

- All M1 commands work on one Ubuntu machine
- Instance is isolated and CPU/memory limited
- Demoable live with `tiny instance *`

Full M1 locks: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md).
