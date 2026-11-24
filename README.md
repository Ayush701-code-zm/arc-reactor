# Tiny AWS

A from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

Not a Docker runtime. Not Kubernetes. Not wrapping AWS APIs.

## Start → end

| Now | Later | End |
|---|---|---|
| **v1 done** (Phases 0–8) | v2–v6 toward “around AWS” | Own create → isolate → limit → network → store → auth → schedule, demoed live with `tiny` |

v1 phases: [`docs/ROADMAP.md`](docs/ROADMAP.md) · After v1 (phased v2–v6): [`docs/VERSIONS.md`](docs/VERSIONS.md)

## Language split

| Language | Role |
|---|---|
| Go | Control plane: CLI, REST, orchestration, SQLite state, scheduler |
| C++20 | Systems core: process, namespaces, cgroups, storage, network, `tinynodeagent` |
| C | eBPF and specialized kernel interfaces (`observability/tinyebpf`) |

Architecture: [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)  
Contracts: [`docs/M1-CONTRACT.md`](docs/M1-CONTRACT.md) … [`docs/M8-CONTRACT.md`](docs/M8-CONTRACT.md) · Proto: [`proto/tiny/v1/`](proto/tiny/v1/)  
Verification (Sonar + Vortex): [`docs/VERIFICATION.md`](docs/VERIFICATION.md)

## Layout

| Path | Contents |
|---|---|
| `control-plane/` | Go module (`tiny`, `tinyd`) |
| `runtime/` | C++20 CMake (`tinyruntimed`, `tinynodeagent`) |
| `observability/tinyebpf/` | C eBPF daemon |
| `proto/tiny/v1/` | gRPC contracts |
| `rules/` | Agent rules (source of truth) |
| `docs/` | Roadmap, versions, contracts, verification |

## Agent rules

Edit `rules/always/` or `rules/scoped/`, then:

```bash
bash scripts/sync-agent-rules.sh
```

That rebuilds `AGENTS.md` and refreshes `.cursor/rules/`. Do not edit `AGENTS.md` by hand.

## Dev environment

Ubuntu Server 24.04 LTS, GCC or Clang (C++20), CMake, Go, gRPC + Protobuf, SQLite, GoogleTest, Git/GitHub.

v1 includes `tinynodeagent` + first-fit scheduling. See [`docs/M8-CONTRACT.md`](docs/M8-CONTRACT.md).

## Demo (v1)

Need **Linux** (Ubuntu). On a Mac use Docker as a **dev host only**.

### Fast path (automated)

```bash
bash scripts/dev-ubuntu-docker.sh
# inside the container:
bash scripts/m1-smoke-linux.sh
bash scripts/m3-smoke-linux.sh
bash scripts/m4-smoke-linux.sh
bash scripts/m5-smoke-linux.sh
bash scripts/m6-smoke-linux.sh
bash scripts/m7-smoke-linux.sh
bash scripts/m8-smoke-linux.sh
```

That builds and exercises create → network → IAM → metadata → eBPF → multi-node agent path. Image import is covered inside later smokes (no separate `m2` script).

### Live demo (you type the commands)

| Step | Who | What |
|---|---|---|
| 1 | you | `bash scripts/dev-ubuntu-docker.sh` (Mac) or open a shell on Ubuntu |
| 2 | you | Build Go + C++ (commands below) |
| 3 | you | Start `tinyruntimed`, then `tinyd`, then (for M8) `tinynodeagent` |
| 4 | you | Auth, image import, instance create, inspect |
| 5 | you | Confirm JSON statuses and (with agent) `NodeID` |

```bash
cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux
(cd control-plane && go build -o bin/tiny ./cmd/tiny && go build -o bin/tinyd ./cmd/tinyd)

export TINY_DB=/src/data/tiny-docker.db
export TINY_RUNTIME_ADDR=127.0.0.1:50051
export TINY_NODE_CONTROL_ADDR=127.0.0.1:50052
export TINY_API_ADDR=127.0.0.1:8080

runtime/build-linux/tinyruntimed --addr 127.0.0.1:50051 >/tmp/tinyruntimed.log 2>&1 &
./control-plane/bin/tinyd >/tmp/tinyd.log 2>&1 &
runtime/build-linux/tinynodeagent --control 127.0.0.1:50052 --runtime 127.0.0.1:50051 >/tmp/tinynodeagent.log 2>&1 &
curl -sf http://127.0.0.1:8080/healthz && echo ok
```

Flow with agent: `tiny` → HTTP → `tinyd` → scheduler → agent Session → local `tinyruntimed` → Linux process.

Full notes (cgroups on Docker Desktop): [`docs/DEV-UBUNTU-DOCKER.md`](docs/DEV-UBUNTU-DOCKER.md)

## Build

```bash
# Go control plane
cd control-plane && go test ./... && go build -o bin/tiny ./cmd/tiny && go build -o bin/tinyd ./cmd/tinyd

# C++ runtime + node agent (use build-linux inside Docker)
cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux && ctest --test-dir runtime/build-linux
```

## Verification (Sonar + Vortex)

```bash
bash scripts/check-sonar-mcp.sh
```

CI: `.github/workflows/sonar.yml` (needs GitHub secret `SONAR_TOKEN`).  
Cursor MCP: `.cursor/mcp.json` (needs `SONARQUBE_TOKEN` + Docker). Details: [`docs/VERIFICATION.md`](docs/VERIFICATION.md).
