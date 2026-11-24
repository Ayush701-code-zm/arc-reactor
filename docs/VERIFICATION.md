# Early verification (Sonar + Vortex)

SonarQube / SonarCloud is the analysis source (bugs, smells, coverage, hotspots).
**Vortex** brings that analysis into the Cursor coding agent earlier via the Sonar MCP server.

v1 is done. Soft quality gate still applies until coverage grows. Full Vortex needs a SonarCloud plan that includes Vortex; without it, MCP still exposes projects, issues, quality gates, and measures.

## Repo pieces (done)

| File | Role |
|---|---|
| `sonar-project.properties` | Project key, sources (`control-plane`, `runtime`, `observability`, `proto`, `docs`), exclusions |
| `.github/workflows/sonar.yml` | CI scan + Go coverage upload (`SONAR_TOKEN` secret) |
| `.cursor/mcp.json` | Sonar MCP Docker image; toolsets `projects,vortex,issues,quality-gates,measures` |
| `rules/always/vortex-verification.md` | Agent must verify while coding; use Vortex/MCP when connected |
| `scripts/check-sonar-mcp.sh` | Local readiness check for token + Docker + MCP config |

## Soft quality gate (cloud)

Default Sonar way gate wants **≥ 80% Coverage on New Code**. We stay on a soft gate until tests catch up.

| Lever | Who | What |
|---|---|---|
| Repo | agent | `sonar.coverage.exclusions` skips C++ / cmd / gRPC stubs from coverage math |
| Cloud | you | Quality gate `V1 Soft`: no hard coverage; keep Security Rating A; duplication ≤ 5% OK |

### Soft gate in SonarCloud (you)

| Step | Action |
|---|---|
| 1 | Open [SonarCloud](https://sonarcloud.io) → org `ayush701-code-zm` |
| 2 | Quality Gates → Create or rename to `V1 Soft` (was `M1 Soft` / `M2 Soft`) |
| 3 | Keep bugs/vulns. **Delete** “Coverage on New Code” (or set to 0%) |
| 4 | Project `arc-reactor` → Administration → Quality Gate → `V1 Soft` |
| 5 | Push so analysis re-runs |

If you skip step 4, GitHub can show **Quality Gate failed** even when the `sonar` Action is green.

## Tests we grow over time

| Package | Status |
|---|---|
| `internal/memory` | unit tests |
| `internal/store` | SQLite lifecycle tests |
| `internal/instance` | service tests with fake runtime |
| `internal/scheduler` | first-fit tests |
| `runtimeclient` gRPC | later (fake server or bufconn) |
| C++ supervisor / cgroup | partial (`supervisor_test`); expand later |
| `tinynodeagent` | later (v2+) |

## You do (cloud + CI)

1. SonarCloud project key must match `sonar-project.properties` (`Ayush701-code-zm_arc-reactor`).
2. GitHub repo secret: `SONAR_TOKEN` (CI scan).
3. Assign soft quality gate `V1 Soft` above.

## Vortex / Sonar in Cursor (MCP)

Repo file: [`.cursor/mcp.json`](../.cursor/mcp.json). It runs `sonarsource/sonarqube-mcp` with **Vortex** plus projects / issues / quality-gates / measures against this workspace.

| Step | Who | Action |
|---|---|---|
| 1 | you | SonarCloud → Account → Security → create a **User token** (not a project token) |
| 2 | you | Put the user token in **gitignored** `.cursor/sonar.env` as one line: `SONARQUBE_TOKEN=…` (Cursor GUI does not load `~/.zshrc`) |
| 3 | you | Optional: also `export SONARQUBE_TOKEN=…` in `~/.zshrc` for terminal checks |
| 4 | you | Docker Desktop running (MCP uses `docker run sonarsource/sonarqube-mcp`) |
| 5 | you | Restart Cursor or toggle the server off/on so it reloads `.cursor/mcp.json` |
| 6 | you | **Customize → MCPs** → confirm `sonarqube-mcp-server` is on / connected |
| 7 | you | Run `bash scripts/check-sonar-mcp.sh` |
| 8 | optional | In chat: `/add-plugin sonarqube` then `/sonar-integrate` (needs Vortex-capable plan) |

**Plan note:** Full Sonar **Vortex** needs SonarCloud **Team (annual)** or **Enterprise** with Vortex enabled. If Vortex tools are skipped, MCP can still expose projects / issues / quality-gates / measures.

Never commit the token. `.cursor/sonar.env` is gitignored. `.cursor/mcp.json` loads it via Docker `--env-file`.

## Local early checks (agent default)

| Area | Command |
|---|---|
| Go | `cd control-plane && go test ./... -coverprofile=coverage.out` and `go vet ./...` |
| C++ | `cmake -S runtime -B runtime/build-linux && cmake --build runtime/build-linux && ctest --test-dir runtime/build-linux` |
| eBPF | `cmake -S observability/tinyebpf -B observability/tinyebpf/build-linux && cmake --build …` (Linux) |
| Proto | `bash scripts/gen-proto.sh` when plugins exist |
| MCP ready | `bash scripts/check-sonar-mcp.sh` |

## Priority

Correctness → FD/process leaks → error paths → security-sensitive Linux calls → dead code → Go↔C++ contract drift → Sonar findings when MCP/CI is available.
