#!/bin/bash
set -u
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOG="$ROOT/hooks/denied.log"
deny(){ printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" >>"$LOG"; printf '%s\n' "{\"permission\":\"deny\",\"user_message\":\"Blocked: $1\",\"agent_message\":\"Safety hook blocked this action ($1). Ask the user, then wait.\"}"; exit 2; }
allow(){ printf '%s\n' '{"permission":"allow"}'; exit 0; }
TMP="$(mktemp)"; trap 'rm -f "$TMP" "$TMP.cmd" "$TMP.meta"' EXIT
cat >"$TMP" || true
[[ -s "$TMP" ]] || deny "empty-input"
python3 - "$TMP" <<'PY'
import json,sys
p=sys.argv[1]; raw=open(p,"rb").read(); tool=fp=pv=cmd=""; has="no"
if raw.strip():
  try: d=json.loads(raw)
  except Exception: d=None
  if isinstance(d,dict):
    def dig(o,*ks):
      c=o
      for k in ks:
        if not isinstance(c,dict) or k not in c: return ""
        c=c[k]
      return "" if c is None or isinstance(c,(dict,list)) else str(c)
    tool=dig(d,"tool_name") or dig(d,"toolName") or ""
    cmd=dig(d,"command") or dig(d,"tool_input","command") or ""
    fp=dig(d,"file_path") or dig(d,"tool_input","file_path") or ""
    pv=dig(d,"path") or dig(d,"tool_input","path") or ""
    if "command" in d or (isinstance(d.get("tool_input"),dict) and "command" in d["tool_input"]): has="yes"
open(p+".meta","w").write("|".join([tool,fp,pv,has])); open(p+".cmd","w").write(cmd)
PY
IFS='|' read -r tool file_path path_value has_command_key <"$TMP.meta"
command="$(cat "$TMP.cmd")"; target="${file_path:-$path_value}"
[[ "$(printf '%s' "$tool" | tr '[:upper:]' '[:lower:]')" == "delete" ]] && deny "delete-tool"
case "$target" in *hooks*|*githooks*|*.cursor/hooks.json*|*/rules/always/safety.md*|*/.env*) deny "protected-path";; esac
lower="$(printf '%s' "$command" | tr '[:upper:]' '[:lower:]')"
[[ -n "$command" ]] || { [[ "$has_command_key" == "yes" ]] && deny "empty-command"; allow; }
printf '%s' "$lower" | grep -Eq 'delete[[:space:]]+from|insert[[:space:]]+into|update[[:space:]].*set|drop[[:space:]]+(table|database)|rm[[:space:]]+-[a-z]*r[a-z]*f|rm[[:space:]]+-[a-z]*f[a-z]*r|(^|[^[:alnum:]_])(kill|pkill|killall)([^[:alnum:]_]|$)|core\.hookspath|--no-verify|reset[[:space:]]+--hard|git[[:space:]]+config' && deny "blocked-command"
allow
