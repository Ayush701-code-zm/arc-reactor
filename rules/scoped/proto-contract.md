---
description: gRPC contract between the Go control plane and the C++ runtime
alwaysApply: false
trigger: glob
globs:
  - "proto/**"
paths:
  - "proto/**"
---

# Proto contract

`proto/tiny/v1/runtime.proto` is the only wire contract. Package `tiny.v1`. Service `RuntimeService`.

Locked M1 decisions: [`docs/M1-CONTRACT.md`](../../docs/M1-CONTRACT.md).
Locked M2 image decisions: [`docs/M2-CONTRACT.md`](../../docs/M2-CONTRACT.md).
Locked M3 volume decisions: [`docs/M3-CONTRACT.md`](../../docs/M3-CONTRACT.md).

| RPC | Use |
|---|---|
| `CreateInstance` | Start supervision for one spec. Control plane supplies `id` and `name` |
| `ListInstances` | Instances this runtime currently knows (live view) |
| `InspectInstance` | One instance by `id` (Go resolves `--name` → id) |
| `StartInstance` | Start a stopped instance by `id` |
| `StopInstance` | SIGTERM, wait 5000 ms, then SIGKILL |
| `DeleteInstance` | Stop if needed, then remove runtime state |
| `AttachVolume` | loop + mount `disk_path` into instance mount ns at `rootfs_path/mnt/<mount_name>` |
| `DetachVolume` | umount + loop detach |

# CreateInstance fields (M1 + M2)

| Direction | Required fields |
|---|---|
| Request | `id`, `name`, `image`, `cpu`, `memory_bytes`, `rootfs_path` (M2) |
| Response | `id`, `status`, `pid`, `error_message` |

- No `command` / `args` yet. Guest runs `/bin/sleep infinity` inside `rootfs_path` after mount ns + `chroot`.
- Go resolves image name → `rootfs_path` before the RPC. C++ does not unpack images.
- `memory_bytes` is a byte count. `cpu` is a whole CPU count (>= 1).
- Instance startup failures: set `status = FAILED` and `error_message`.
- Transport / RPC failures: gRPC status codes, not `error_message` alone.
- Control plane owns metadata. Runtime owns the process. Do not send SQL rows across this API.
- Do not add a side channel for these calls. M1 transport is gRPC only (no HTTP JSON API).
- Change the `.proto` first, run `scripts/gen-proto.sh` (commit Go stubs), rebuild C++ (CMake regenerates C++ stubs). Do not hand-edit generated files.
- Keep field numbers. Add new fields. Do not renumber a field that a milestone already uses.
- Generated code is not the source of truth. The `.proto` file is.
