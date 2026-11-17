# Dev host: Ubuntu in Docker (Option C)

Use Docker only as a **Linux host** so you can exercise pid namespaces and cgroup v2 on a Mac.

This is **not** the Tiny instance runtime. Instances are still created by our C++ supervisor (`/bin/sleep infinity` + pid ns + cgroup v2). Never create a Tiny instance with `docker run`.

## You do

```bash
bash scripts/dev-ubuntu-docker.sh
```

That builds `tiny-aws-dev:24.04` and opens a privileged Ubuntu 24.04 shell with the repo mounted at `/src`.

Inside the container:

```bash
bash scripts/m1-smoke-linux.sh
```

That builds Go + C++, starts `tinyruntimed`, and runs create → list → inspect → stop → start → delete.

## Manual demo inside the container

```bash
# Use build-linux inside Docker so it does not clash with macOS runtime/build
cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux
(cd control-plane && go build -o bin/tiny ./cmd/tiny)

runtime/build-linux/tinyruntimed --addr 127.0.0.1:50051 &
export TINY_DB=/src/data/tiny-docker.db
export TINY_RUNTIME_ADDR=127.0.0.1:50051

./control-plane/bin/tiny instance create --name my-server --image alpine --cpu 1 --memory 512MB
./control-plane/bin/tiny instance list
./control-plane/bin/tiny instance inspect my-server
./control-plane/bin/tiny instance stop my-server
./control-plane/bin/tiny instance start my-server
./control-plane/bin/tiny instance delete my-server
```

## Why `--privileged` and private cgroupns

M1 needs cgroup v2 writes (`+cpu` / `+memory` then `cpu.max` / `memory.max`) and pid namespaces.

Use `--privileged --cgroupns=private` (not host cgroupns). On Docker Desktop, host cgroupns often leaves `cpu`/`memory` undelegated under `/sys/fs/cgroup/docker/<id>`, so create fails.

If an old container is still open, exit it and run `bash scripts/dev-ubuntu-docker.sh` again.

Before enabling controllers, the runtime moves tasks into `tiny.init`. cgroup v2 rejects `+cpu`/`+memory` while the parent cgroup still has member processes (`Operation not supported`).

### Docker Desktop limit

Docker Desktop on Mac often still returns `Operation not supported` for `cgroup.subtree_control`. For local smoke, the runtime may start the process with pid namespace only and set:

`error_message: cgroup_limits=skipped: ...`

That is **not** full M1 exit criteria. Enforce real limits with:

```bash
TINY_REQUIRE_CGROUP=1 bash scripts/m1-smoke-linux.sh   # bare Ubuntu / CI
```

GitHub Actions workflow: `.github/workflows/m1-linux.yml` (ubuntu-24.04 runner).

## Related

- Contract: [`docs/M1-CONTRACT.md`](M1-CONTRACT.md)
- Verification: [`docs/VERIFICATION.md`](VERIFICATION.md)
