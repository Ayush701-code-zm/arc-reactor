# Milestone 5 — locked IAM contract

Phase 5 is done (v1). Do not change these locks without an explicit new decision. Post-v1: [`docs/VERSIONS.md`](VERSIONS.md).

## Phase status

| Phase | Status |
|---|---|
| 1–5 | done |
| 6–8 | done (v1) |
| Post-v1 | [`docs/VERSIONS.md`](VERSIONS.md) |

## Ownership

| Piece | Owner |
|---|---|
| Users, passwords, sessions, owner checks, CLI auth | **Go + SQLite** |
| C++ runtime | **identity-stateless** (no user tokens on gRPC in M5) |

## Identity

| Piece | Rule |
|---|---|
| Permanent id | user UUID |
| Human handle | unique `username` |
| Password | Argon2id hash in SQLite (never plaintext) |
| Access token | opaque; store SHA-256 hash; Bearer on HTTP; TTL **15m** |
| Refresh token | opaque; store SHA-256 hash; rotate on refresh; TTL **7d** |
| CLI | `Authorization: Bearer <access>` from env `TINY_TOKEN` |

## Authz

| Rule | M5 behavior |
|---|---|
| Default | **owner-only** on create/mutate/delete/list (list = own resources) |
| Admin | `role=admin` column reserved; no admin override API in M5 |
| Unowned rows | bootstrap admin **adopts** rows with empty `owner_id` |

## Bootstrap

| Path | Rule |
|---|---|
| Primary | `tiny auth bootstrap --username U --password P` |
| Fallback | if zero users and `TINY_BOOTSTRAP_USER` + `TINY_BOOTSTRAP_PASSWORD` set at tinyd start, seed once |
| Race | single txn: insert only if `COUNT(users)=0`; loser gets HTTP 409 |

## Public (no Bearer)

| HTTP | Role |
|---|---|
| `GET /healthz` | liveness |
| `POST /v1/auth/bootstrap` | first user |
| `POST /v1/auth/login` | username + password → tokens |
| `POST /v1/auth/refresh` | refresh → new access + refresh |

## Authenticated

| HTTP | Role |
|---|---|
| `POST /v1/auth/logout` | revoke refresh (+ current access) |
| `GET /v1/auth/whoami` | current user |
| All instance/image/volume/network routes | Bearer required |

## Out of Phase 5

OAuth, long-lived API keys as default login, RBAC product UI, forwarding user identity on Go→C++ gRPC, MFA, org/tenants.
