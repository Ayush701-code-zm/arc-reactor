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

Default Sonar way gate wants **≥ 80% Coverage on New Code**.

Two levers (use both until tests catch up):

| Lever | Who | What |
|---|---|---|
| Repo | agent | `sonar.coverage.exclusions` skips C++/cmd/gRPC from coverage math (no C++ coverage report yet) |
| Cloud | you | Assign quality gate `M1 Soft` without a hard coverage condition |

### Soft gate in SonarCloud (you)

| Step | Action |
|---|---|
| 1 | Open [SonarCloud](https://sonarcloud.io) → org `ayush701-code-zm` |
| 2 | Quality Gates → Create `M1 Soft` |
| 3 | Keep bugs/vulns conditions. **Delete** “Coverage on New Code” (or set to 0%) |
| 4 | Project `arc-reactor` → Administration → Quality Gate → `M1 Soft` |
| 5 | Push again so analysis re-runs |

If you skip step 4, GitHub still shows **Quality Gate failed** even when `sonar` Actions is green.

Later: remove coverage exclusions path-by-path as real tests land. Do not jump back to 80% until store, instance, runtime, and gRPC have tests.

## Tests we grow over time

| Package | Status |
|---|---|
| `internal/memory` | unit tests |
| `internal/store` | SQLite lifecycle tests |
| `internal/instance` | service tests with fake runtime client |
| `runtimeclient` gRPC | later (needs fake server or bufconn) |
| C++ supervisor / cgroup | partial (`supervisor_test`); expand later |

## You do (cloud)

1. SonarCloud project key must match `sonar-project.properties` (`Ayush701-code-zm_arc-reactor`).
2. GitHub repo secret: `SONAR_TOKEN` (for CI scan).
3. Assign soft quality gate above for M1.

## Vortex / Sonar in Cursor (MCP)

Repo file: [`.cursor/mcp.json`](../.cursor/mcp.json). It runs the local Sonar MCP Docker image with `vortex` + `projects` toolsets against this workspace.

| Step | Who | Action |
|---|---|---|
| 1 | you | SonarCloud → Account → Security → create a **User token** (not a project token) |
| 2 | you | In your Mac shell profile (`~/.zshrc`): `export SONARQUBE_TOKEN=your_user_token` then `source ~/.zshrc` |
| 3 | you | Docker Desktop must be running (MCP uses `docker run sonarsource/sonarqube-mcp`) |
| 4 | you | Restart Cursor so it loads `.cursor/mcp.json` |
| 5 | you | Cursor Settings → MCP → confirm `sonarqube-mcp-server` is connected |
| 6 | optional | In chat: `/add-plugin sonarqube` then `/sonar-integrate` (official plugin path; needs Vortex-capable plan) |

**Plan note:** Full Sonar Vortex needs SonarCloud **Team (annual)** or **Enterprise** with Vortex enabled. If Vortex tools are skipped, MCP can still expose projects/issues/quality-gates from SonarCloud.

Never commit the token. Only `${env:SONARQUBE_TOKEN}` is referenced from MCP config.

## Local early checks (agent default)

| Area | Command |
|---|---|
| Go | `cd control-plane && go test ./... -coverprofile=coverage.out` and `go vet ./...` |
| C++ | `cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux && ctest --test-dir runtime/build-linux` |
| Proto | Edit `proto/tiny/v1/runtime.proto`, then `bash scripts/gen-proto.sh` when plugins exist |

## Priority

Correctness → FD/process leaks → error paths → security-sensitive Linux calls → dead code → Go↔C++ contract drift.
