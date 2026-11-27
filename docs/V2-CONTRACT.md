# Version 2 — locked multi-node real contract

v2 is open. Phase **V2.1** only until its exit criteria pass. v1 stays done. Quality bar: thin spine + deep spikes ([`docs/VERSIONS.md`](VERSIONS.md)).

## Phase status

| Phase | Status |
|---|---|
| V2.1 Image on node | **CURRENT** |
| V2.2 Agent owns ops | locked |
| V2.3 Two-node proof | locked |

## V2.1 locks

### Rootfs materialization

| Rule | Lock |
|---|---|
| Who | **Node-local** create path (`tinynodeagent` → local store). Not tinyd pushing a control-host path by luck |
| Bytes on Session | **Never.** No image/rootfs payload on the NodeControl stream |
| Create input | Logical **image id** (control-plane UUID). Agent materializes instance rootfs from a **READY** local image |
| Distribution | Import / place / replicate are **separate** from create. Not implicit in create |

### Node-local store

| Rule | Lock |
|---|---|
| Root | `TINY_NODE_IMAGE_ROOT/<image-id>/` |
| Image id | Same globally stable control-plane image UUID (no per-node image id) |
| READY | Image must be imported/placed on that node and marked READY before create |
| Layout | `…/<image-id>/rootfs/` usable tree; status recorded in control-plane SQLite `node_images` |

### EnsureRootfs

| Rule | Lock |
|---|---|
| What | Internal step of **node-side CreateInstance** (agent), after validation, before `tinyruntimed` create |
| Separate RPC | **No.** Scheduler/agent must not call a prep RPC during create |
| Idempotent | Safe to retry create after a failed runtime start if the instance rootfs already exists |
| Prerequisite | Selected node already has that image READY locally |

Interpretation of “local CreateInstance lifecycle”: the create hop on the **chosen node** (agent), not Go inventing a remote filesystem. tinyd only checks READY and sends `image_id`.

### Import source

| Rule | Lock |
|---|---|
| Preferred | Control-plane `artifact_path` (`.tar.gz`) |
| Fallback | Existing unpacked `rootfs_path` if archive missing |
| Precedence | Archive wins when both exist |
| Place-on-node | Separate API/lifecycle; materializes under `TINY_NODE_IMAGE_ROOT/<image-id>/` |

### Missing image

| Rule | Lock |
|---|---|
| Behavior | **Hard-fail** create immediately |
| No wait | No sync, download, or import during create |
| Error | Structured, names **image id** and **node id** |

### V2.1 demo

| Rule | Lock |
|---|---|
| Shape | Two agents on **one** Linux host |
| Isolation | Distinct node id, runtime addr, and `TINY_NODE_IMAGE_ROOT` each |
| Proof | Image READY on node A only → create on A works; create forced/scheduled to B fails clearly |
| Deferred | Real multi-host and network-failure drills → **V2.3** |

## Smoke

`scripts/v21-smoke-linux.sh` — two agents, place on one image root only, create fails on the other, create succeeds on the READY node.

## Out of V2.1

Agent-proxied volume/network (V2.2), two-host / reconnect / capacity hard-fail smoke suite (V2.3), overlay, Rust, guest metadata, streaming image bytes on Session.

## Ownership

| Piece | Owner |
|---|---|
| Image registry, place-on-node orchestration, `node_images`, create READY check | **Go tinyd** |
| `TINY_NODE_IMAGE_ROOT`, Import/place materialization on disk, EnsureRootfs, create→runtime | **C++ `tinynodeagent`** |
| Process start | **C++ `tinyruntimed`** (unchanged contract fields; rootfs path is node-local) |
