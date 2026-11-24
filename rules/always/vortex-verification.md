---
description: Vortex-style early verification in the coding loop. Always apply.
alwaysApply: true
trigger: always_on
---

# Early verification

SonarQube already has information about the codebase (bugs, smells, coverage, security hotspots).
Vortex brings that analysis into the AI coding workflow earlier.
So the agent receives project context and verification feedback while coding, instead of waiting for a late pipeline run.

| Piece | Role |
|---|---|
| SonarQube | Source of analysis |
| Vortex | Makes that analysis available to the coding agent earlier |

Verification is part of the coding loop, not a final gate.

# Before large edits

Gather local context first:

- Architecture rules and module boundaries
- Public interfaces and the Go to C++ contract
- Failing tests
- Compiler and linter errors
- Existing SonarQube, Vortex, or similar findings if present

# After each meaningful change

Run the cheapest relevant check:

| Area | Check |
|---|---|
| Go | `go test` / `go vet` on touched packages |
| C++ | Build touched targets. Run GoogleTest if present |
| Proto | Confirm both Go and C++ still compile against the contract |

If SonarQube, Vortex, or similar analysis output exists in the workspace or tools, read it and apply findings before continuing.

# Priority order

1. Correctness
2. FD and process leaks
3. Error paths
4. Security-sensitive Linux calls
5. Dead code
6. Go to C++ contract drift

# How to report findings

Do not dump raw scanner noise. Convert each finding into:

1. What is wrong
2. Why it matters for Tiny AWS
3. Smallest fix

# Sonar / Vortex wiring in this repo

| Piece | Status |
|---|---|
| `sonar-project.properties` | Present (v1.0.0; sources include `observability/`) |
| `.github/workflows/sonar.yml` | Present (runs when `SONAR_TOKEN` is set) |
| `.cursor/mcp.json` | Present; toolsets `projects,vortex,issues,quality-gates,measures` |
| `scripts/check-sonar-mcp.sh` | Local readiness check (token + Docker + config) |
| `docs/VERIFICATION.md` | Setup steps for CI Sonar and Cursor MCP / Vortex |
| SonarCloud user token | User must export `SONARQUBE_TOKEN` (user token, not project token) |
| Vortex subscription | Requires SonarCloud Team annual / Enterprise with Vortex; otherwise MCP may only expose non-Vortex tools |

When the Sonar MCP namespace is connected, prefer its issue / quality-gate / Vortex tools after meaningful edits, then still run compiler and tests.

If MCP/Vortex is not connected, say what is missing in one line, then use the compiler and tests as early feedback. Point the user at `bash scripts/check-sonar-mcp.sh` and `docs/VERIFICATION.md`.

Do not block coding on a live Vortex session unless the user asks to finish that setup.
