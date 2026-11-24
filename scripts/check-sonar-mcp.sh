#!/bin/bash
# Read-only readiness check for Sonar MCP + Vortex wiring.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
fail=0

ok() { printf 'ok %s\n' "$1"; }
bad() { printf 'FAIL %s\n' "$1"; fail=1; }
note() { printf 'note %s\n' "$1"; }

if [[ -f .cursor/mcp.json ]]; then
  ok "mcp.json present"
else
  bad "mcp.json missing"
fi

if [[ -f .cursor/mcp.json ]] && grep -q 'vortex' .cursor/mcp.json; then
  ok "vortex toolset listed in mcp.json"
else
  bad "vortex toolset missing from mcp.json"
fi

if [[ -f .cursor/mcp.json ]] && grep -q 'sonar.env' .cursor/mcp.json; then
  ok "mcp.json uses .cursor/sonar.env via Docker --env-file"
else
  bad "mcp.json missing sonar.env --env-file wiring"
fi

if [[ -f sonar-project.properties ]]; then
  ok "sonar-project.properties present"
else
  bad "sonar-project.properties missing"
fi

if [[ -f .github/workflows/sonar.yml ]]; then
  ok "sonar CI workflow present"
else
  bad "sonar CI workflow missing"
fi

if [[ -f .cursor/sonar.env ]] && grep -q '^SONARQUBE_TOKEN=' .cursor/sonar.env; then
  ok ".cursor/sonar.env has SONARQUBE_TOKEN"
else
  bad ".cursor/sonar.env missing SONARQUBE_TOKEN (gitignored; see docs/VERIFICATION.md)"
fi

if [[ -n "${SONARQUBE_TOKEN:-}" ]]; then
  ok "SONARQUBE_TOKEN is set in this shell (optional for terminal checks)"
else
  note "SONARQUBE_TOKEN unset in shell (OK if .cursor/sonar.env exists for MCP)"
fi

if command -v docker >/dev/null 2>&1; then
  ok "docker CLI present"
  if docker info >/dev/null 2>&1; then
    ok "docker daemon reachable"
  else
    bad "docker daemon not reachable (start Docker Desktop)"
  fi
else
  bad "docker CLI missing"
fi

note "Full Vortex needs SonarCloud Team annual / Enterprise. Without it, MCP may still expose issues and quality gates."
note "After export + Docker: restart Cursor and confirm sonarqube-mcp-server in Customize → MCPs."

if [[ "$fail" -ne 0 ]]; then
  printf '\nSonar/Vortex MCP not ready (%s check(s) failed). See docs/VERIFICATION.md\n' "$fail"
  exit 1
fi

printf '\nSonar/Vortex MCP wiring looks ready for Cursor.\n'
exit 0
