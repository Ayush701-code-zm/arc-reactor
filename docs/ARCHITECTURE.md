# Tiny AWS architecture

Language ownership and system shape. Phases live in [`docs/ROADMAP.md`](ROADMAP.md). Current focus is Phase 1 (Milestone 1 Tiny EC2).

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
| Specialized | C | eBPF programs, specialized kernel interfaces. Not Milestone 1 |
| Host | Ubuntu Server 24.04 LTS | Kernel resource management, networking and filesystem primitives |

Initial language decision:

- C++ is the primary systems language
- Go handles the control plane
- Do not introduce Rust into the initial implementation

## System shape (Milestone 1)

```
tiny (Go CLI)
  -> tinyd (Go control plane: validate, SQLite, orchestrate)
    -> gRPC (proto/tiny/v1)
      -> tinyruntimed (C++ runtime: isolate, limit, supervise)
        -> Linux process (+ namespaces/cgroups as decided for M1)
```

- Control plane owns identity, desired state, and orchestration (SQLite).
- Runtime owns the live process and Linux primitives (pid namespace + cgroup v2 in M1).
- They talk only through `proto/tiny/v1/runtime.proto`.
- Locked M1 contract: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md).

## Dev environment

- Ubuntu Server 24.04 LTS
- GCC or Clang with C++20
- CMake
- Go
- gRPC and Protobuf
- SQLite
- Git and GitHub
- GoogleTest

Initial deployment: everything on one Linux machine first. Multi-node is Phase 8.

## Hard bans

| Ban | Meaning |
|---|---|
| Docker as instance runtime | No Docker, containerd, or runc to create instances |
| Kubernetes | No pods, controllers, or kubelet |
| AWS API wrappers | No AWS SDK / API calls to pretend this is the cloud |
| Rust in v1 | No Rust in the initial implementation |
| Phase jump | Do not build Phase 2+ until Phase 1 exit criteria are met, unless the user opens that phase |

## See also

- Roadmap (now vs later vs end): [`docs/ROADMAP.md`](ROADMAP.md)
- Current milestone detail: [`docs/MILESTONE-1.md`](MILESTONE-1.md)
