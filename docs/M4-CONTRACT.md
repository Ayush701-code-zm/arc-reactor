# Milestone 4 — locked network contract

Phase 4 is open. Phases 1–3 stay complete. Do not change these locks without an explicit new decision.

## Phase status

| Phase | Status |
|---|---|
| 1 Tiny EC2 | done |
| 2 Images | done |
| 3 Storage | done |
| 4 Networking | CURRENT |
| 5+ | locked later |

## Ownership

| Piece | Owner |
|---|---|
| Network metadata, CIDR, IP allocation, CLI, HTTP, SQLite | **Go** |
| netns, veth, bridge, interface config | **C++** |

## Identity

| Piece | Rule |
|---|---|
| Primary id | UUID |
| Human handle | unique `--name` |
| Bridge name | `tn` + first 12 hex of network UUID (≤15 chars) |
| Host veth | `th` + first 12 hex of instance UUID |
| Guest iface | `eth0` inside instance netns |
| NIC count | **one** attachment per instance in M4 |

## Addressing

| Piece | Rule |
|---|---|
| CIDR | set on `network create --cidr` (example `10.10.0.0/24`) |
| Gateway | first usable host in CIDR (`.1`); assigned on the bridge |
| Instance IP | Go allocates next free host; C++ applies `guest_ip/prefix` on `eth0` |
| Egress / NAT | **out of M4** (isolated private only) |

## CLI / HTTP

| Command | HTTP |
|---|---|
| `tiny network create --name NAME --cidr CIDR` | `POST /v1/networks` |
| `tiny network list` | `GET /v1/networks` |
| `tiny network inspect NAME` | `GET /v1/networks/{name}` |
| `tiny network delete NAME` | `DELETE /v1/networks/{name}` |
| `tiny network attach --network NAME --instance INST` | `POST /v1/networks/{name}/attach` |
| `tiny network detach --network NAME --instance INST` | `POST /v1/networks/{name}/detach` |
| `tiny instance create ... --network NAME` | create then attach (same as separate attach) |

## Boot (C++)

Every RUNNING instance enters a **netns** at start (plus existing pid + mount). Loopback is brought up. Attach moves a veth peer into that netns.

## gRPC

| RPC | Role |
|---|---|
| `AttachNetwork` | ensure bridge, create veth, move peer into instance netns, set IPs |
| `DetachNetwork` | tear down veth; leave bridge if other attachments remain |

## Failure compensate

If gRPC attach succeeds and SQLite fails: call `DetachNetwork`, return error, no attachment row.

## Out of Phase 4

NAT/egress, security groups, multi-NIC, IPv6, DNS, overlay, remote VPC peering.
