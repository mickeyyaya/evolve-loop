#!/usr/bin/env bash
# scripts/hooks/doc-deletion-guard.sh — PreToolUse kernel hook (v1.0, cycle-90)
#
# Enforces the Knowledge Stewardship Rule (AGENTS.md §13 / Plan §5D):
#   docs/** and knowledge-base/** content must NEVER be deleted — only archived.
#
# Allowed operations:
#   mv docs/X  knowledge-base/research/archived-YYYY-MM-DD/X  (canonical archival)
#   Any operation when EVOLVE_ALLOW_DOC_DELETE=1               (operator escape)
#
# Denied operations:
#   rm targeting docs/** or knowledge-base/**
#   mv from docs/** or knowledge-base/** to a non-archival destination
#
# Contract (matches role-gate.sh / research-quota-gate.sh shape):
#   stdin:  JSON {tool_name, tool_input}
#   stderr: denial message on deny
#   rc=0:   allow
#   rc=2:   deny

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
GUARDS_LOG="${EVOLVE_GUARDS_LOG:-$REPO_ROOT/.evolve/guards.log}"

mkdir -p "$(dirname "$GUARDS_LOG")" 2>/dev/null || true

_log() {
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  printf '[%s] [doc-deletion-guard] %s\n' "$ts" "$*" >> "$GUARDS_LOG" 2>/dev/null || true
}

_deny() {
  local msg="$1"
  _log "DENY: $msg"
  printf '[doc-deletion-guard] DENY: %s\n' "$msg" >&2
  printf '[doc-deletion-guard] Approved alternative: mv <file> knowledge-base/research/archived-%s/<file>\n' \
    "$(date -u +%Y-%m-%d)" >&2
  printf '[doc-deletion-guard] Emergency bypass: export EVOLVE_ALLOW_DOC_DELETE=1\n' >&2
  exit 2
}

# Read full stdin payload
INPUT=$(cat 2>/dev/null || true)
[ -z "$INPUT" ] && exit 0

# No jq → no enforcement (degrade gracefully)
if ! command -v jq >/dev/null 2>&1; then
  _log "WARN: jq not on PATH; doc-deletion-guard skipped"
  exit 0
fi

# Operator escape hatch
if [ "${EVOLVE_ALLOW_DOC_DELETE:-0}" = "1" ]; then
  _log "WARN: EVOLVE_ALLOW_DOC_DELETE=1 — guard bypassed (operator override)"
  exit 0
fi

TOOL_NAME=$(printf '%s' "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null || true)

case "$TOOL_NAME" in
  Bash)
    CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null || true)
    [ -z "$CMD" ] && exit 0

    # Detect: rm targeting docs/ or knowledge-base/
    # Matches patterns like: rm docs/foo.md  rm -rf docs/  rm knowledge-base/bar
    if printf '%s' "$CMD" | grep -qE '(^|[[:space:];|&(])rm[[:space:]]+(-[a-zA-Z]+ )*([^ ]*[[:space:]]+)*([^ ]*\b)?(docs|knowledge-base)/'; then
      _deny "rm targeting docs/ or knowledge-base/ is forbidden — archive instead. Command: ${CMD:0:200}"
    fi

    # Detect: mv from docs/ or knowledge-base/ to a non-archival destination
    # Allow ONLY when destination matches knowledge-base/research/archived-YYYY-MM-DD/
    if printf '%s' "$CMD" | grep -qE '(^|[[:space:];|&(])mv[[:space:]]'; then
      # Check if source is docs/ or knowledge-base/
      if printf '%s' "$CMD" | grep -qE 'mv[[:space:]]+(-[a-zA-Z]+ )*[^ ]*?(docs|knowledge-base)/'; then
        # Archival destination check: must contain knowledge-base/research/archived-YYYY-MM-DD/
        if ! printf '%s' "$CMD" | grep -qE 'knowledge-base/research/archived-[0-9]{4}-[0-9]{2}-[0-9]{2}/'; then
          _deny "mv from docs/ or knowledge-base/ must target knowledge-base/research/archived-YYYY-MM-DD/. Command: ${CMD:0:200}"
        fi
        _log "ALLOW: archival mv detected: ${CMD:0:200}"
      fi
    fi
    ;;

  Edit|Write)
    # Edit/Write modify file contents in place; they cannot delete files.
    # Pass through — the guard targets deletion operations only.
    exit 0
    ;;

  *)
    # All other tools: passthrough
    exit 0
    ;;
esac

exit 0
