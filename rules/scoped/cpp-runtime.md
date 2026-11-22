---
description: C++20 runtime boundaries for Tiny AWS
alwaysApply: false
trigger: glob
globs:
  - "runtime/**"
  - "**/*.cc"
  - "**/*.cpp"
  - "**/*.h"
  - "**/*.hpp"
  - "**/CMakeLists.txt"
paths:
  - "runtime/**"
  - "**/*.cc"
  - "**/*.cpp"
  - "**/*.h"
  - "**/*.hpp"
  - "**/CMakeLists.txt"
---

# C++ runtime

Locked M1 contract: [`docs/M1-CONTRACT.md`](../../docs/M1-CONTRACT.md).
Locked M3 volume contract: [`docs/M3-CONTRACT.md`](../../docs/M3-CONTRACT.md).
Locked M4 network contract: [`docs/M4-CONTRACT.md`](../../docs/M4-CONTRACT.md).

- Language is C++20. Build with CMake. GCC or Clang.
- Public headers live in `include/tiny/`. Sources live in `src/`. Tests live in `tests/`.
- This process is the gRPC server. It receives a spec and owns the Linux process.
- Foundation behavior is argument checks and not-implemented. Do not call `fork`, `clone`, `exec`, or write under `/sys/fs/cgroup` until the user asks for process creation.
- When that step starts (M1): enter **pid** namespace only, apply cgroup v2 `cpu.max` and `memory.max`, run the image's fixed stub command, supervise the child.
- Stop sequence: `SIGTERM`, wait **5000 ms**, then `SIGKILL`. Status is `STOPPING` then `STOPPED`.
- Startup failures: return response with `FAILED` and `error_message`. Transport failures: gRPC status.
- Do not shell out to Docker, `systemd-run`, or `runc`.
- M2: mount namespace + `chroot`. M3: loop + mount volumes. M4: per-instance netns + veth + bridge. Still no uts/user namespaces, no NAT/egress product, and no eBPF.
- A stub must say it is a stub. Do not return success for a process you did not start.
- GoogleTest is the framework once a test needs fixtures. Until then a single test binary is enough. The test target must pass under `ctest`.
