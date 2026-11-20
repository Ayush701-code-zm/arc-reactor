# Tiny AWS roadmap

Source of truth for phases. **Phase 3 Storage is done.** Phase 4+ stays locked until you explicitly open it.

## What Tiny AWS is

A from-scratch mini cloud built on Linux primitives. We implement the infrastructure mechanisms ourselves.

## What it is not

- Not a Docker-based runtime
- Not a Kubernetes dependency or distro
- Not wrapping AWS APIs
- Not a production AWS competitor

Hard engineering rule: no Docker-based runtime, no Kubernetes dependency, and no wrapping AWS APIs.

## Phase checklist

| Phase | Name | Status |
|---|---|---|
| 0 | Foundation | done |
| 1 | Tiny EC2 | done |
| 2 | Images | done |
| 3 | Storage | done |
| 4 | Networking | locked later |
| 5 | IAM | locked later |
| 6 | Metadata | locked later |
| 7 | eBPF observability | locked later |
| 8 | Multi-node | locked later |

## Phase gate

Do not implement Phase 2+ features until Phase 1 exit criteria are met, unless the user explicitly opens the next phase.

If the user asks for a later-phase feature early, remind them of this gate and ask whether to stay on Milestone 1 or explicitly open that phase.

---

## PHASE 0 — Repository foundation (done / in progress)

Build:

- Repo layout
- Cursor/agent rules system (always + scoped + sync + `AGENTS.md`)
- C++ runtime skeleton
- Go control plane skeleton
- Protobuf/gRPC communication contract between Go and C++

Exit criteria:

- Project compiles as skeletons
- Contract exists
- Rules and roadmap are durable in-repo

---

## PHASE 1 — Milestone 1: Tiny EC2 (done)

Goal: a working Linux instance manager on one machine.

Primary UX:

```bash
tiny instance create \
  --name my-server \
  --image alpine \
  --cpu 1 \
  --memory 512MB
```

Required flow:

1. Receive request through Go CLI
2. Send it to Go control plane
3. Validate instance configuration
4. Ask C++ runtime to create the instance
5. Prepare an isolated Linux environment
6. Apply CPU and memory limits
7. Start and supervise the process
8. Return instance ID and status

Must also support:

- `tiny instance list`
- `tiny instance inspect <name>`
- `tiny instance stop <name>`
- `tiny instance start <name>`
- `tiny instance delete <name>`

Includes:

- Go CLI + control plane validation/orchestration
- SQLite metadata/state
- gRPC to C++ runtime
- pid namespace + cgroup v2 CPU/memory limits
- process start / supervise / stop / start / delete

Excludes:

- Full virtual networking product
- Block storage product
- IAM product depth
- eBPF
- Multi-node scheduling
- Other AWS-like services beyond instance lifecycle

Exit criteria:

- All M1 commands work reliably on one Ubuntu machine
- Instance is actually isolated and CPU/memory limited
- No orphan processes or ghost DB rows on failure paths
- Demoable live with `tiny instance *`

Details: [`docs/MILESTONE-1.md`](MILESTONE-1.md)  
Locked contract: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md)

---

## PHASE 2 — Images (done)

Build:

- Rootfs / image artifacts (AMI-like local images)
- Local image store (`.tar.gz` import + unpack)
- Boot instance from prepared `rootfs_path` (mount ns + `chroot`)

Locked contract: [`docs/M2-CONTRACT.md`](M2-CONTRACT.md)

Depends on: Phase 1 stable (done).

---

## PHASE 3 — Tiny storage (EBS-like) (done)

Build:

- Volumes create/delete (Go sparse `disk.img` + SQLite)
- Attach/detach to instances (C++ loop + mount into instance mount ns)
- Guest path `/mnt/<volume-name>`

Locked contract: [`docs/M3-CONTRACT.md`](M3-CONTRACT.md)

Depends on: Phase 1 + Phase 2 images (done).

---

## PHASE 4 — Tiny networking (VPC-like)

Build:

- Virtual networking with Linux primitives
- veth, bridges, private IPs, basic routing
- Instance network isolation basics

Depends on: Phase 1 stable. Prefer after storage/images unless the user prioritizes network.

---

## PHASE 5 — IAM + API hardening

Build:

- Users, keys/tokens, policies
- Authenticated REST API + CLI auth
- Authorization checks on instance (and later resource) APIs

Depends on: stable control plane from Phase 1.

---

## PHASE 6 — Metadata service

Build:

- Instance metadata endpoint style service
- Instance identity/config readable from inside/outside as designed

Depends on: Phase 1 + enough network identity to make metadata useful.

---

## PHASE 7 — Observability hooks (eBPF in C)

Build:

- eBPF programs only where needed
- Specialized kernel interfaces
- Better monitoring/visibility for runtime/node agent

Depends on: working runtime. Do not start in Milestone 1.

---

## PHASE 8 — Multi-node

Build:

- Node agent on each machine
- Capacity reporting
- Scheduler places instances across nodes
- Control plane remains Go; runtime remains C++ per node

Depends on: single-node Tiny EC2 + enough networking/storage for multi-node to matter.

---

## End state

A coherent mini-cloud control plane + systems core that we can demo and explain:

- Compute: multi-instance Tiny EC2 with isolation, limits, supervision
- Storage: attachable volumes + image-backed boots
- Network: virtual networks between instances
- Identity: authenticated API + CLI (IAM)
- Multi-node: node agents + scheduler
- Ops: inspect/monitor/restart/delete as first-class behavior

End goal in one line: own create → isolate → limit → network → store → auth → schedule on Linux primitives, and demo it live with `tiny`.
