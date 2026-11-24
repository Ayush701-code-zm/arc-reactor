# Milestone 6 — locked metadata contract

Phase 6 is done (v1). Do not change these locks without an explicit new decision. Post-v1: [`docs/VERSIONS.md`](VERSIONS.md).

## Phase status

| Phase | Status |
|---|---|
| 1–6 | done |
| 7–8 | done (v1) |
| Post-v1 | [`docs/VERSIONS.md`](VERSIONS.md) |

## Ownership

| Piece | Owner |
|---|---|
| Metadata document, HTTP paths, RUNNING/owner gates | **Go tinyd** |
| Guest→host forward (link-local / DNAT) | **C++ later** (not required for M6 host API) |

## Access (M6)

| Path | Rule |
|---|---|
| Canonical JSON | `GET /v1/instances/{name}/metadata` (Bearer + owner) |
| AWS-style | `GET /v1/instances/{name}/meta-data/` and leaf paths |
| By host veth | `GET /v1/metadata/by-veth/{vethHost}` (Bearer + owner of that instance) |
| Guest in-netns listener | **out of M6** |

## Document

Canonical flat JSON fields:

| Field | Source |
|---|---|
| `instance-id` | SQLite instance id |
| `name` | SQLite name |
| `owner-id` | SQLite owner_id |
| `image` | SQLite image |
| `local-ipv4` | applied `network_attachments.guest_ip` only |
| `mac` / iface bind key | applied `veth_host` (host-side veth identity) |
| `network-id` | applied attachment network_id |
| `availability` | only if status == RUNNING; else HTTP 409 |

AWS-style leaves (text/plain body):

- `instance-id`, `local-hostname` (= name), `local-ipv4`, `public-ipv4` (empty in M6)

## Lifecycle

- Readable only while instance is **RUNNING**
- Stable identity fields always from SQLite
- Network fields only from **successful** attachment rows, never desired-but-unapplied config
- Metadata failure must **not** fail instance create or trigger delete

## Out of Phase 6

In-guest `169.254.169.254`, DNAT/forward in C++, IMDSv2 tokens, user-data blobs, IAM role credentials on metadata.
