# Tiny AWS

A from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

Not a Docker runtime. Not Kubernetes. Not wrapping AWS APIs.

## Start → end

| Now | Later | End |
|---|---|---|
| Phase 1 Tiny EC2: create / list / inspect / stop / start / delete on one Ubuntu machine | Images, storage, networking, IAM, metadata, eBPF, multi-node | Own create → isolate → limit → network → store → auth → schedule, demoed live with `tiny` |

Full phases and exit criteria: [`docs/ROADMAP.md`](docs/ROADMAP.md)

## Language split

| Language | Role |
|---|---|
| Go | Control plane: CLI, REST, orchestration, SQLite state |
| C++20 | Systems core: process, namespaces, cgroups, later storage/network/node agent |
| C | eBPF and specialized kernel interfaces only (not Milestone 1) |

Architecture: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)  
Current milestone: [`docs/MILESTONE-1.md`](docs/MILESTONE-1.md)  
M1 contract: [`docs/M1-CONTRACT.md`](docs/M1-CONTRACT.md) · [`proto/tiny/v1/runtime.proto`](proto/tiny/v1/runtime.proto) (gRPC only)

## Layout

| Path | Contents |
|---|---|
| `control-plane/` | Go module (`tiny`, `tinyd`) |
| `runtime/` | C++20 CMake project |
| `proto/tiny/v1/` | gRPC contract |
| `rules/` | Agent rules (source of truth) |
| `docs/` | Roadmap, architecture, milestone |

## Agent rules

Edit `rules/always/` or `rules/scoped/`, then:

```bash
bash scripts/sync-agent-rules.sh
```

That rebuilds `AGENTS.md` and refreshes `.cursor/rules/`. Do not edit `AGENTS.md` by hand.

## Dev environment

Ubuntu Server 24.04 LTS, GCC or Clang (C++20), CMake, Go, gRPC + Protobuf, SQLite, GoogleTest, Git/GitHub.

One Linux machine first. Multi-node is later (Phase 8).

## Build (skeleton)

```bash
# Go control plane
cd control-plane && go test ./... && go build -o bin/tiny ./cmd/tiny && go build -o bin/tinyd ./cmd/tinyd

# C++ runtime
cmake -S runtime -B runtime/build && cmake --build runtime/build && ctest --test-dir runtime/build
```

Process create/stop with pid namespace + cgroup v2 needs **Linux**. On macOS the runtime builds and returns a clear failure for create.

```bash
./runtime/build/tinyruntimed --addr 127.0.0.1:50051
TINY_DB=./data/tiny.db ./control-plane/bin/tiny instance create --name my-server --image alpine --cpu 1 --memory 512MB
```

Verification / Sonar: [`docs/VERIFICATION.md`](docs/VERIFICATION.md)

**You do for SonarCloud/Vortex:** create project key `tiny-aws`, set `SONAR_TOKEN`, connect Vortex/Sonar in Cursor.

## Linux on Mac (Option C)

Docker is a **dev host only**, not the instance runtime:

```bash
bash scripts/dev-ubuntu-docker.sh
# inside the container:
bash scripts/m1-smoke-linux.sh
```

Details: [`docs/DEV-UBUNTU-DOCKER.md`](docs/DEV-UBUNTU-DOCKER.md)
