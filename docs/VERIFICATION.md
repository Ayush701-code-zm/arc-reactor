# Early verification (Sonar + Vortex style)

SonarQube/SonarCloud is the analysis source (bugs, smells, coverage, hotspots).
Vortex (or similar) brings that analysis into the coding agent earlier.
Until Vortex is connected in Cursor, use compiler and tests as the early loop.

## Repo pieces (done)

| File | Role |
|---|---|
| `sonar-project.properties` | Sonar project key, sources, exclusions |
| `.github/workflows/sonar.yml` | CI scan + Go coverage upload |
| `rules/always/vortex-verification.md` | Agent must verify while coding |

## Milestone 1 quality gate (soft)

Default Sonar way gate wants **≥ 80% Coverage on New Code**. That fails while M1 still grows tests.

| Step | Who | Action |
|---|---|---|
| 1 | you | SonarCloud → Quality Gates → Create `M1 Soft` |
| 2 | you | Keep reliability/security conditions. Remove or set **Coverage on New Code** to **0%** (or omit it) |
| 3 | you | Project `arc-reactor` → Project Settings → Quality Gate → assign `M1 Soft` |
| 4 | you | Re-run analysis (push or Actions → sonar) |

Later: raise coverage conditions as Go/C++ tests grow. Do not jump straight back to 80% until store, instance, and runtime paths have real tests.

## Tests we grow over time

| Package | Status |
|---|---|
| `internal/memory` | unit tests |
| `internal/store` | SQLite lifecycle tests |
| `internal/instance` | service tests with fake runtime client |
| `runtimeclient` gRPC | later (needs fake server or bufconn) |
| C++ supervisor / cgroup | partial (`supervisor_test`); expand later |

## You do (cloud / Vortex)

1. SonarCloud project key must match `sonar-project.properties` (`Ayush701-code-zm_arc-reactor`).
2. Repo secret: `SONAR_TOKEN`.
3. Assign soft quality gate above for M1.
4. Connect Vortex (or Sonar in Cursor) so findings appear while coding.

## Local early checks (agent default)

| Area | Command |
|---|---|
| Go | `cd control-plane && go test ./... -coverprofile=coverage.out` and `go vet ./...` |
| C++ | `cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux && ctest --test-dir runtime/build-linux` |
| Proto | Edit `proto/tiny/v1/runtime.proto`, then `bash scripts/gen-proto.sh` when plugins exist |

## Priority

Correctness → FD/process leaks → error paths → security-sensitive Linux calls → dead code → Go↔C++ contract drift.
