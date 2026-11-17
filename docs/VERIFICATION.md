# Early verification (Sonar + Vortex style)

SonarQube/SonarCloud is the analysis source (bugs, smells, coverage, hotspots).
Vortex (or similar) brings that analysis into the coding agent earlier.
Until Vortex is connected in Cursor, use compiler and tests as the early loop.

## Repo pieces (done)

| File | Role |
|---|---|
| `sonar-project.properties` | Sonar project key, sources, exclusions |
| `.github/workflows/sonar.yml` | Optional CI scan (needs secrets) |
| `rules/always/vortex-verification.md` | Agent must verify while coding |

## You do (cloud / Vortex)

1. Create a SonarCloud (or SonarQube) project with key `tiny-aws`.
2. Add repo secrets if using GitHub Actions: `SONAR_TOKEN`, and org/host as required.
3. Connect Vortex (or Sonar in Cursor) to that project so findings appear while coding.

## Local early checks (agent default)

| Area | Command |
|---|---|
| Go | `cd control-plane && go test ./...` and `go vet ./...` |
| C++ | `cmake -S runtime -B runtime/build && cmake --build runtime/build && ctest --test-dir runtime/build` |
| Proto | Edit `proto/tiny/v1/runtime.proto`, then `bash scripts/gen-proto.sh` when plugins exist |

## Priority

Correctness → FD/process leaks → error paths → security-sensitive Linux calls → dead code → Go↔C++ contract drift.
