# Milestone 2 — locked image contract

Phase 2 is done (v1). Do not change these locks without an explicit new decision. Post-v1: [`docs/VERSIONS.md`](VERSIONS.md).

## Phase status

| Phase | Status |
|---|---|
| 1 Tiny EC2 | done |
| 2 Images | done |
| 3–8 | done (v1) |
| Post-v1 | [`docs/VERSIONS.md`](VERSIONS.md) |

## Artifact

| Lock | Value |
|---|---|
| Format | `.tar.gz` only (squashfs later) |
| Flow | Import file → store artifact → unpack to rootfs dir → boot from that dir |
| Ownership | **Go** imports, extracts, stores metadata. **C++** only receives prepared `rootfs_path` |

## Identity

| Piece | Rule |
|---|---|
| Primary id | UUID |
| Human handle | unique `--name` |
| On disk | `TINY_IMAGE_ROOT/<uuid>/artifact` and `TINY_IMAGE_ROOT/<uuid>/rootfs/` |
| Default root | `TINY_IMAGE_ROOT` = `~/.tiny-aws/images` (override with env) |

## CLI / HTTP

| Command | HTTP |
|---|---|
| `tiny image import --name NAME --file PATH` | `POST /v1/images` (multipart or JSON path for M2 local) |
| `tiny image list` | `GET /v1/images` |
| `tiny image inspect NAME` | `GET /v1/images/{name}` |
| `tiny image delete NAME` | `DELETE /v1/images/{name}` |
| `tiny instance create --image NAME ...` | Go resolves name → `rootfs_path`, then gRPC |

## gRPC

`CreateInstanceRequest` adds `rootfs_path` (required for Phase 2 boots).

## Boot (C++)

| Lock | Value |
|---|---|
| Isolation | Existing pid ns + cgroup v2, plus **mount namespace** |
| Root | `chroot(rootfs_path)` then exec guest init |
| Guest command (M2) | Prefer `/bin/sleep infinity` inside rootfs; else fail with clear error |
| Later | `pivot_root` postponed |

## Out of Phase 2

SquashFS, remote registry, `pivot_root`, networking product, storage product.
