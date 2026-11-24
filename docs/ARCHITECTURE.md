# Tiny AWS architecture

Language ownership and system shape. v1 phases: [`docs/ROADMAP.md`](ROADMAP.md). Post-v1 versions: [`docs/VERSIONS.md`](VERSIONS.md). **v1 is done.**

## What it is

A from-scratch mini cloud on Linux primitives. We implement the mechanisms ourselves.

## What it is not

- Not a Docker-based runtime
- Not a Kubernetes dependency or distro
- Not wrapping AWS APIs
- Not a production AWS competitor

## Language ownership (locked)

| Layer | Language | Owns |
|---|---|---|
| Control plane | Go | CLI and command handling, REST API, IAM and authentication, resource orchestration, scheduler, metadata and state |
| Systems core | C++20 | Linux process management, instance runtime, namespaces and cgroups, filesystem and block storage ops, virtual networking, node agent, resource monitoring |
| Specialized | C | eBPF programs and specialized kernel interfaces (`observability/tinyebpf`) |
| Host | Ubuntu Server 24.04 LTS | Kernel resource management, networking and filesystem primitives |

Initial language decision:

- C++ is the primary systems language
- Go handles the control plane
- Do not introduce Rust into the initial implementation

## System shape (v1)

```
tiny (Go CLI)
  -> tinyd (Go control plane: validate, SQLite, orchestrate, schedule)
    -> NodeControl Session gRPC (when agents connected)
         -> tinynodeagent
              -> RuntimeService gRPC -> tinyruntimed -> Linux
    -> RuntimeService gRPC (legacy zero-agent / volume-network dial)
         -> tinyruntimed -> Linux
```

- Control plane owns identity, desired state, orchestration, and scheduling (SQLite).
- Runtime owns the live process and Linux primitives.
- Node agent owns registration, capacity heartbeat, and create/lifecycle proxy to local runtime.
- CLI → `tinyd` is HTTP REST. Contracts: [`proto/tiny/v1/runtime.proto`](../proto/tiny/v1/runtime.proto), [`proto/tiny/v1/node.proto`](../proto/tiny/v1/node.proto).
- Historical M1 detail: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md). Multi-node: [`docs/M8-CONTRACT.md`](M8-CONTRACT.md).

## Dev environment

- Ubuntu Server 24.04 LTS
- GCC or Clang with C++20
- CMake
- Go
- gRPC and Protobuf
- SQLite
- Git and GitHub
- GoogleTest

v1 supports self-registering node agents. True multi-host product work is post-v1 ([`docs/VERSIONS.md`](VERSIONS.md)).

## Hard bans

| Ban | Meaning |
|---|---|
| Docker as instance runtime | No Docker, containerd, or runc to create instances |
| Kubernetes | No pods, controllers, or kubelet |
| AWS API wrappers | No AWS SDK / API calls to pretend this is the cloud |
| Rust in v1 | No Rust in the initial implementation |
| Version / phase jump | Do not build post-v1 work until the user opens that version and phase ([`docs/VERSIONS.md`](VERSIONS.md)) |

## See also

- Roadmap (now vs later vs end): [`docs/ROADMAP.md`](ROADMAP.md)
- Current milestone detail: [`docs/MILESTONE-1.md`](MILESTONE-1.md)
