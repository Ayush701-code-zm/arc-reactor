# Milestone 3 — locked volume contract

Phase 3 storage slice is complete. Phase 1 and Phase 2 stay complete. Do not change these locks without an explicit new decision.

## Phase status

| Phase | Status |
|---|---|
| 1 Tiny EC2 | done |
| 2 Images | done |
| 3 Storage | done |
| 4+ | locked later |

## Exit criteria (met)

| Check | Evidence |
|---|---|
| volume create/list/inspect/delete | CLI + HTTP + SQLite |
| attach/detach loop+mount | live Docker demo + `scripts/m3-smoke-linux.sh` |
| instance delete detaches volumes first | `instance.Service.Delete` → `DetachAllForInstance` |
| list/inspect keep `RootfsPath` | SQLite `instances.rootfs_path` |
| CI | `.github/workflows/m1-linux.yml` runs M3 smoke |

## Ownership

| Piece | Owner |
|---|---|
| Volume metadata, CLI, HTTP, SQLite | **Go** |
| Sparse disk file create (`disk.img`) | **Go** |
| loop setup, mkfs (first attach), mount/umount in instance mount ns | **C++** |

## Identity and disk

| Piece | Rule |
|---|---|
| Primary id | UUID |
| Human handle | unique `--name` |
| On disk | `TINY_VOLUME_ROOT/<uuid>/disk.img` (sparse) |
| Default root | `TINY_VOLUME_ROOT` = `~/.tiny-aws/volumes` |

## CLI / HTTP

| Command | HTTP |
|---|---|
| `tiny volume create --name NAME --size SIZE` | `POST /v1/volumes` |
| `tiny volume list` | `GET /v1/volumes` |
| `tiny volume inspect NAME` | `GET /v1/volumes/{name}` |
| `tiny volume delete NAME` | `DELETE /v1/volumes/{name}` |
| `tiny volume attach --volume NAME --instance INST` | `POST /v1/volumes/{name}/attach` |
| `tiny volume detach --volume NAME` | `POST /v1/volumes/{name}/detach` |

Guest mount path: `/mnt/<volume-name>` (inside chroot).

## gRPC

| RPC | Role |
|---|---|
| `AttachVolume` | loop + mount `disk_path` into instance mount ns at `rootfs/mnt/<mount_name>` |
| `DetachVolume` | umount + detach loop |

## Failure compensate

If gRPC attach succeeds and SQLite update fails: call `DetachVolume`, return error, leave volume `AVAILABLE`.

## Out of Phase 3

Snapshots, multi-attach, remote block store, encryption, resize online, `pivot_root` changes.
