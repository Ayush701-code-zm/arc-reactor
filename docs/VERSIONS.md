# Tiny AWS versions (after v1)

Phases 0–8 are **v1**. They built the mechanism spine on Linux primitives.

This file is the source of truth for major versions after v1. Each version has **one job**, split into a few **phases** (same style as v1). Finish one phase before the next. Do not invent extra phases “to be thorough.”

Goal shape: **around AWS** as a teachable mini-cloud (EC2 / VPC / EBS / IAM / multi-node mental model). Not AWS API parity. Not a production AWS competitor.

## Quality bar (locked)

**Thin spine + deep spikes.**

| Keep thin | Make deep |
|---|---|
| One clear create / schedule / agent path | V2: image on node, agent-owned ops, two-node proof |
| No AWS API cosplay, no feature zoo | V3–V4: real cross-node network + durable storage |
| No mid-flight language rewrite (Rust stays out unless a later version opens it) | V5–V6: policy/quotas + failure drills |

Deferred v1 items (guest `169.254`, custom command/args, squashfs, XDP, live migration, tinyd HA, Rust) stay **unplaced** until a version or phase explicitly takes them. They do not auto-enter v2–v6.

## Version checklist

| Version | One job | Phases | Status |
|---|---|---|---|
| **v1** | Mechanism spine | 0–8 | **done** |
| **v2** | Make multi-node real | V2.1–V2.3 | **CURRENT** (phase **V2.1** only) |
| **v3** | Cross-node network | V3.1–V3.2 | locked until opened |
| **v4** | Durable storage product | V4.1–V4.2 | locked until opened |
| **v5** | Control plane product | V5.1–V5.2 | locked until opened |
| **v6** | Operator confidence (optional) | V6.1–V6.2 | locked until opened |

Locked order: **v2 → v3 → v4 → v5 → (optional) v6**.

Rule: one version = one product lie you can stop telling. One phase = one exit you can demo.

## Gate

| Rule | Meaning |
|---|---|
| Open version explicitly | User must say open v2 (or v3…) before implementation |
| Open one phase | Inside an open version, implement the current phase only |
| Contract first | One contract per version (`docs/V2-CONTRACT.md`, …) with locked phase subsections. Write it when the version is opened, before code |
| No mash | Do not pull the next phase or next version “while we are here” |
| AWS parity | Never a version goal |

Pace: phase exit criteria must pass before the next phase starts.

## v1 (done) — mechanism spine

Phased delivery: Tiny EC2, images, volumes, networks, IAM, metadata, eBPF observability, node agent + first-fit schedule.

Known v1 limits (honest):

- Create can stamp `node_id` and go through `tinynodeagent`, but image/rootfs locality across hosts is not a product yet
- Volume/network attach may still assume the single `TINY_RUNTIME_ADDR` dial
- Overlay, tinyd HA, live migration, queues stay out

---

## v2 — Make multi-node real

**Kill the lie:** “multi-node is done because `NodeID` exists.”

Depends on: v1 Phase 8 agent + scheduler.

Keep out of every v2 phase: overlay/VXLAN product, IAM depth, snapshots, HA tinyd.

| Phase | Achieve | Exit when |
|---|---|---|
| **V2.1 Image on node** | Chosen node has a usable rootfs before create (sync, pull, or node-local import) | Create on a non-local node does not depend on a shared host path by luck |
| **V2.2 Agent owns ops** | Instance lifecycle plus the volume/network attach and detach that instance needs go through `tinynodeagent` for that `node_id` | tinyd does not need a second runtime dial for those instance ops |
| **V2.3 Two-node proof** | Two agents, first-fit, reconnect, capacity hard-fail | Smoke shows two distinct node ids, a full node hard-fails create, and a reconnecting agent comes back |

### V2.1 — Image on node

Build:

- A defined way for the scheduled node to hold the image rootfs before `CreateInstance`
- Fail create if that rootfs is not ready on the chosen node

Exit criteria:

- A create that targets a second node uses that node’s rootfs path
- Missing rootfs fails clearly (no silent use of another machine’s path)

### V2.2 — Agent owns ops

Build:

- Create, stop, start, delete, inspect already on the agent stay the only lifecycle path when agents are connected
- Volume attach/detach and network attach/detach for that instance go through the agent session to the local runtime

Exit criteria:

- With ≥1 agent connected, those ops do not use tinyd’s direct `TINY_RUNTIME_ADDR` dial
- Failure on the agent path marks the instance failed or returns the error; no half-attached silent success

### V2.3 — Two-node proof

Build:

- Smoke with two `tinynodeagent` processes (two node ids) against one tinyd
- Placement picks a node with free CPU and memory
- Hard-fail when neither node fits
- Agent disconnect then reconnect updates connected state

Exit criteria:

- Smoke script passes on Linux
- Inspect shows `NodeID` equal to one of the two registered ids
- A request larger than both nodes returns `no node with capacity`

---

## v3 — Cross-node network

**Kill the lie:** “VPC-like only works on one host.”

Depends on: v2 complete (real multi-node).

Keep out: full transit-gateway story, snapshots, control-plane HA.

Transport choice (routed or overlay) is locked in `docs/V3-CONTRACT.md` when v3 opens. Do not decide it in code first.

| Phase | Achieve | Exit when |
|---|---|---|
| **V3.1 Connect** | Instances on different nodes can reach each other | Two-node guest ping or HTTP works in smoke |
| **V3.2 Allow/deny** | Basic security-group shaped rules (not an AWS clone) | An allow list or default-deny rule is enforced and demoable |

### V3.1 — Connect

Build:

- A path so two instances on two nodes share a Tiny network and can exchange packets
- Control-plane records which node owns each attachment

Exit criteria:

- Smoke: two guests, two node ids, one successful reachability check
- Same-node attach from v1 still works

### V3.2 — Allow/deny

Build:

- A small rule set on a network or instance (allow listed peers or ports, or default deny plus allows)
- Enforcement on the data path chosen in V3.1

Exit criteria:

- Allowed flow succeeds
- Denied flow fails
- Rule change is visible via CLI or API inspect

---

## v4 — Durable storage product

**Kill the lie:** “volumes are just mount helpers.”

Depends on: v2 (node-local disk truth) and v3 (cross-node network done so storage is not built on a lie about where the instance lives).

Locked artifact: **snapshot**. Image-from-volume stays out unless a later contract reopens it.

Keep out: multi-attach, full EBS API surface.

| Phase | Achieve | Exit when |
|---|---|---|
| **V4.1 Restart-safe volumes** | Volume lifecycle is clear; the disk survives instance stop/start as designed | Attach semantics are documented and covered by smoke |
| **V4.2 Snapshot** | Create a volume snapshot and restore it to a volume | CLI or API smoke: snapshot create, then restore, then see data |

### V4.1 — Restart-safe volumes

Build:

- Documented lifecycle: create, attach, detach, delete, and what stop/start does to the disk
- Disk bytes remain on the owning node across instance stop and start

Exit criteria:

- Smoke writes a marker file on the volume, stops and starts the instance, and reads the marker back
- Delete of an attached volume fails or detaches first, by the written rule

### V4.2 — Snapshot

Build:

- Snapshot create from a volume
- Restore snapshot into a new or existing volume (pick one in the v4 contract and stick to it)
- Snapshot list and delete

Exit criteria:

- Smoke: write data, snapshot, change or destroy the source content, restore, read the snapshotted data back

---

## v5 — Control plane product

**Kill the lie:** “auth is login only.”

Depends on: v2 complete. v3 and v4 should be done first so policy and quotas apply to real multi-node resources.

Keep out: “match AWS IAM,” org/tenant product UI.

| Phase | Achieve | Exit when |
|---|---|---|
| **V5.1 Authz + quotas** | Roles or policies beyond owner-only; simple CPU, memory, or count quotas | Policy deny and quota reject are visible in the API |
| **V5.2 Audit + backup + errors** | Audit-ish event log, DB backup/restore, stable error JSON | Restore drill works and error bodies share one shape |

### V5.1 — Authz + quotas

Build:

- A small policy or role check on instance create (and one mutate path)
- A quota (instance count, or sum of CPU, or sum of memory) enforced in tinyd before placement

Exit criteria:

- A forbidden caller gets a deny, not a created instance
- A caller over quota gets a reject before runtime create

### V5.2 — Audit + backup + errors

Build:

- Append-only style event rows or log for create, delete, login, and deny
- A documented SQLite backup and restore procedure
- One error JSON shape for API failures (`error` plus a stable code field)

Exit criteria:

- Restore from backup brings users and instance rows back on a fresh DB path
- Two different failures (not found, forbidden) return the same JSON keys

---

## v6 (optional) — Operator confidence

**Kill the lie:** “it works if nobody fails.”

Depends on: v2–v5. Skip this version if you stop at a teaching cloud.

Keep out: new product surfaces that belong in v3–v5.

| Phase | Achieve | Exit when |
|---|---|---|
| **V6.1 Drain + signals** | A draining node is not a placement target; inspect already exposes useful eBPF or capacity signals | A draining node receives no new instances |
| **V6.2 Failure drills** | Scripted agent crash, disk-full style failure, and disconnect | A written smoke set records pass or fail for each drill |

### V6.1 — Drain + signals

Build:

- Node state `draining` (CLI or API) that the scheduler skips
- Inspect or node list shows capacity and, when eBPF is up, the existing observability fields

Exit criteria:

- Create with only a draining node connected hard-fails or uses another ready node
- Clearing drain allows placement again

### V6.2 — Failure drills

Build:

- Smoke or script notes for: agent dies during create, agent disconnect, and a full-disk or failed runtime create
- Expected control-plane result for each (failed row, retry, or clean error)

Exit criteria:

- Each drill is runnable and documented with the expected status
- No orphan “RUNNING” row when the runtime create never succeeded

---

## See also

- v1 phase history: [`docs/ROADMAP.md`](ROADMAP.md)
- v1 multi-node contract: [`docs/M8-CONTRACT.md`](M8-CONTRACT.md)
- Architecture: [`docs/ARCHITECTURE.md`](ARCHITECTURE.md)
