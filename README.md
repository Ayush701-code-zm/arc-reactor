# Tiny AWS

A from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

Not a Docker runtime. Not Kubernetes. Not wrapping AWS APIs.

## Start → end

| Now | Later | End |
|---|---|---|
| Phase 5 IAM (CURRENT); 1–4 done | Metadata, eBPF, multi-node | Own create → isolate → limit → network → store → auth → schedule, demoed live with `tiny` |

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

## Demo (Milestone 1)

Need **Linux** (Ubuntu). On a Mac use Docker as a **dev host only**.

### Fast path (automated)

```bash
bash scripts/dev-ubuntu-docker.sh
# inside the container:
bash scripts/m1-smoke-linux.sh
# Phase 4 networking / Phase 5 IAM:
bash scripts/m4-smoke-linux.sh
bash scripts/m5-smoke-linux.sh
```

That builds everything, starts `tinyruntimed` + `tinyd`, runs create → list → inspect → stop → start → delete. M4 adds network create/attach.

### Live demo (you type the commands)

Inside the Ubuntu container (or on a real Ubuntu box):

| Step | Who | What |
|---|---|---|
| 1 | you | `bash scripts/dev-ubuntu-docker.sh` (Mac) or open a shell on Ubuntu |
| 2 | you | Build Go + C++ (commands below) |
| 3 | you | Start `tinyruntimed`, then `tinyd` |
| 4 | you | Run the six `tiny instance` commands |
| 5 | you | Confirm JSON statuses: RUNNING → STOPPED → RUNNING → deleted |

```bash
cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux
(cd control-plane && go build -o bin/tiny ./cmd/tiny && go build -o bin/tinyd ./cmd/tinyd)

export TINY_DB=/src/data/tiny-docker.db          # or another path on bare Ubuntu
export TINY_RUNTIME_ADDR=127.0.0.1:50051
export TINY_API_ADDR=127.0.0.1:8080

runtime/build-linux/tinyruntimed --addr 127.0.0.1:50051 >/tmp/tinyruntimed.log 2>&1 &
./control-plane/bin/tinyd >/tmp/tinyd.log 2>&1 &
curl -sf http://127.0.0.1:8080/healthz && echo ok

./control-plane/bin/tiny instance create --name my-server --image alpine --cpu 1 --memory 512MB
./control-plane/bin/tiny instance list
./control-plane/bin/tiny instance inspect my-server
./control-plane/bin/tiny instance stop my-server
./control-plane/bin/tiny instance start my-server
./control-plane/bin/tiny instance delete my-server

# Phase 3 volumes (needs a RUNNING instance + image rootfs)
./control-plane/bin/tiny volume create --name data --size 64MB
./control-plane/bin/tiny volume attach --volume data --instance my-server
./control-plane/bin/tiny volume detach --volume data
./control-plane/bin/tiny volume delete data
```

Flow: `tiny` → HTTP → `tinyd` → gRPC → `tinyruntimed` → Linux process.

Full notes (cgroups on Docker Desktop): [`docs/DEV-UBUNTU-DOCKER.md`](docs/DEV-UBUNTU-DOCKER.md)

## Build (skeleton)

```bash
# Go control plane
cd control-plane && go test ./... && go build -o bin/tiny ./cmd/tiny && go build -o bin/tinyd ./cmd/tinyd

# C++ runtime (use build-linux inside Docker)
cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux && ctest --test-dir runtime/build-linux
```

Verification / Sonar: [`docs/VERIFICATION.md`](docs/VERIFICATION.md)
