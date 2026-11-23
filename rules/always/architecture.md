---
description: Language ownership, hard bans, and phase gate for Tiny AWS. Always apply.
alwaysApply: true
trigger: always_on
---

# What Tiny AWS is

A from-scratch mini cloud on Linux primitives. We implement the infrastructure mechanisms ourselves.

# What it is not

- Not a Docker-based runtime
- Not a Kubernetes dependency or distro
- Not wrapping AWS APIs
- Not a production AWS competitor

# Ownership

Go is the control plane. C++20 is the systems core. C is only for eBPF and other specialized kernel interfaces, and C is out of Milestone 1.

| Language | Owns |
|---|---|
| Go | CLI and command handling, REST API, IAM and authentication, resource orchestration, scheduler, metadata and state management, the gRPC client |
| C++20 | Linux process management, instance runtime, namespaces and cgroups, filesystem and block storage ops, virtual networking, node agent, resource monitoring, the gRPC server |
| C | eBPF programs and specialized kernel interfaces. Not Milestone 1 |

Initial language decision: C++ is the primary systems language. Go handles the control plane. Do not introduce Rust into the initial implementation.

The control plane and runtime talk only through `proto/tiny/v1/runtime.proto`.

Roadmap and system shape: [`docs/ROADMAP.md`](../../docs/ROADMAP.md), [`docs/ARCHITECTURE.md`](../../docs/ARCHITECTURE.md).

# Hard bans

| Ban | Meaning |
|---|---|
| Docker as the instance runtime | Do not create instances with Docker, containerd, or runc. Using Docker as an Ubuntu *dev host* (`scripts/dev-ubuntu-docker.sh`) is allowed for M1 verification on a Mac |
| Kubernetes | Do not add pods, controllers, or kubelet |
| AWS API wrappers | Do not call AWS, and do not vendor an AWS SDK to pretend this is the cloud |
| Rust in v1 | Do not add Rust to the initial implementation |
| eBPF in Milestone 1 | Do not add C or eBPF until Phase 7 |
| Scope jump | Do not add images, networking product, storage product, IAM depth, metadata, or multi-node before Phase 1 exit criteria, unless the user opens that phase |
| Fake completion | Prefer a stub that returns not-implemented over code that pretends an instance was started |

# Phase gate

Current phase: **Phase 6 — Metadata** (Phases 1–5 complete).

Do not implement Phase 7+ until Phase 6 exit criteria are met, unless the user explicitly opens the next phase.

If the user asks for a later-phase feature early, remind them of the roadmap gate and ask whether to stay on Phase 6 or explicitly open that phase.

# Build tools

Host target is Ubuntu Server 24.04 LTS. Compile C++ with GCC or Clang in C++20 mode. Use CMake, Go, gRPC, Protobuf, SQLite, GoogleTest, Git, and GitHub.

Initial deployment: one Linux machine first. Multi-node is Phase 8.

Keep each change compilable.
