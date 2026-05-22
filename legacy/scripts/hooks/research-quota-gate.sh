#!/usr/bin/env bash
# legacy/scripts/hooks/research-quota-gate.sh — PreToolUse research quota gate.
#
# Counts research-tool calls per agent and denies over-quota invocations.
# Cycle A foundation (research-as-tool-c1).
#
# Contract (matches role-gate.sh shape):
#   stdin:  JSON {tool_name, tool_input}
#   stdout: JSON block message on deny
#   rc=0:   allow
#   rc=2:   deny
#
# Env overrides:
#   EVOLVE_ALLOW_DEEP_RESEARCH=1    — lift cap; records deep_overrides counter
#   EVOLVE_RESEARCH_HOOK_DISABLED=1 — telemetry-only no-op (counters still increment)
#   EVOLVE_CYCLE_STATE_FILE         — override cycle-state path (for tests)
#   EVOLVE_GUARDS_LOG               — override guards.log path

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

CYCLE_STATE_FILE="${EVOLVE_CYCLE_STATE_FILE:-$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json}"
GUARDS_LOG="${EVOLVE_GUARDS_LOG:-$EVOLVE_PROJECT_ROOT/.evolve/guards.log}"
PROFILES_DIR="$EVOLVE_PLUGIN_ROOT/.evolve/profiles"

_log() {
  printf '[research-quota-gate] %s\n' "$*" >> "$GUARDS_LOG" 2>/dev/null || true
}

# Read full stdin (PreToolUse hook receives the tool JSON on stdin)
INPUT=$(cat 2>/dev/null || true)
[ -z "$INPUT" ] && exit 0

# No jq = no enforcement; always allow
if ! command -v jq >/dev/null 2>&1; then
  exit 0
fi

# Determine bucket from tool_name
TOOL_NAME=$(printf '%s' "$INPUT" | jq -r '.tool_name // ""' 2>/dev/null || true)

case "$TOOL_NAME" in
  WebSearch|web_search) BUCKET="web_search" ;;
  WebFetch|web_fetch)   BUCKET="web_fetch" ;;
  Bash)
    CMD=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // ""' 2>/dev/null || true)
    if printf '%s' "$CMD" | grep -q 'kb-search\.sh'; then
      BUCKET="kb_search"
    else
      # Untracked Bash call — not a research tool
      exit 0
    fi
    ;;
  *) exit 0 ;;
esac

# No cycle-state = no enforcement; always allow
if [ ! -f "$CYCLE_STATE_FILE" ]; then
  exit 0
fi

# Read active agent
AGENT=$(jq -r '.active_agent // "unknown"' "$CYCLE_STATE_FILE" 2>/dev/null || true)
[ -z "$AGENT" ] || [ "$AGENT" = "null" ] && AGENT="unknown"

# Read quota from agent profile (EVOLVE_PLUGIN_ROOT so worktree edits are visible)
QUOTA=""
profile_file="$PROFILES_DIR/${AGENT}.json"
if [ -f "$profile_file" ]; then
  QUOTA=$(jq -r --arg b "$BUCKET" '.research_quota[$b] // empty' "$profile_file" 2>/dev/null || true)
fi

# Defaults if profile missing or quota unset
if [ -z "$QUOTA" ] || [ "$QUOTA" = "null" ] || [ "$QUOTA" = "0" ]; then
  case "$BUCKET" in
    web_search) QUOTA=3 ;;
    web_fetch)  QUOTA=5 ;;
    kb_search)  QUOTA=20 ;;
    *)          QUOTA=3 ;;
  esac
fi

# Acquire file lock (mkdir-based, bash 3.2 compatible, portable)
_LOCK_DIR="${CYCLE_STATE_FILE}.rqg.lock"
_lock_acquired=0
_attempts=0
while ! mkdir "$_LOCK_DIR" 2>/dev/null; do
  _attempts=$((_attempts + 1))
  if [ "$_attempts" -gt 200 ]; then
    _log "WARNING: lock timeout agent=$AGENT bucket=$BUCKET"
    break
  fi
  sleep 0.05 2>/dev/null || true
done
[ "$_attempts" -le 200 ] && _lock_acquired=1

# Read current counter from cycle-state (inside lock)
CURRENT=$(jq -r --arg a "$AGENT" --arg b "$BUCKET" \
  '(.research_usage // {}) | (.[$a] // {}) | (.[$b] // 0)' \
  "$CYCLE_STATE_FILE" 2>/dev/null || true)
[ -z "$CURRENT" ] || [ "$CURRENT" = "null" ] && CURRENT=0

# Decide: allow or deny?
ALLOW=1
DEEP=0

if [ "${EVOLVE_RESEARCH_HOOK_DISABLED:-0}" = "1" ]; then
  ALLOW=1
elif [ "${CURRENT:-0}" -ge "${QUOTA:-3}" ] 2>/dev/null; then
  if [ "${EVOLVE_ALLOW_DEEP_RESEARCH:-0}" = "1" ]; then
    ALLOW=1
    DEEP=1
  else
    ALLOW=0
  fi
fi

# Increment counter on allowed calls (inside lock)
if [ "$ALLOW" = "1" ]; then
  if [ "$DEEP" = "1" ]; then
    _updated=$(jq -c --arg a "$AGENT" --arg b "$BUCKET" \
      '.research_usage = ((.research_usage // {})
        | .[$a] = ((.[$a] // {})
          | .[$b] = ((.[$b] // 0) + 1)
          | .deep_overrides = ((.deep_overrides // 0) + 1)))' \
      "$CYCLE_STATE_FILE" 2>/dev/null) || _updated=""
    if [ -n "$_updated" ]; then
      printf '%s\n' "$_updated" > "${CYCLE_STATE_FILE}.tmp.$$" \
        && mv -f "${CYCLE_STATE_FILE}.tmp.$$" "$CYCLE_STATE_FILE" 2>/dev/null || true
    fi
    _log "deep-override: agent=$AGENT bucket=$BUCKET quota=$QUOTA"
  else
    _updated=$(jq -c --arg a "$AGENT" --arg b "$BUCKET" \
      '.research_usage = ((.research_usage // {})
        | .[$a] = ((.[$a] // {})
          | .[$b] = ((.[$b] // 0) + 1)))' \
      "$CYCLE_STATE_FILE" 2>/dev/null) || _updated=""
    if [ -n "$_updated" ]; then
      printf '%s\n' "$_updated" > "${CYCLE_STATE_FILE}.tmp.$$" \
        && mv -f "${CYCLE_STATE_FILE}.tmp.$$" "$CYCLE_STATE_FILE" 2>/dev/null || true
    fi
    _new_count=$((CURRENT + 1))
    _log "allow: agent=$AGENT bucket=$BUCKET usage=$_new_count quota=$QUOTA"
  fi
fi

# Release lock
if [ "$_lock_acquired" = "1" ]; then
  rmdir "$_LOCK_DIR" 2>/dev/null || true
fi

if [ "$ALLOW" = "0" ]; then
  _log "deny: agent=$AGENT bucket=$BUCKET usage=$CURRENT quota=$QUOTA"
  printf '{"action":"block","message":"Research quota exceeded: %s has used %s/%s %s calls. Set EVOLVE_ALLOW_DEEP_RESEARCH=1 to override."}\n' \
    "$AGENT" "$CURRENT" "$QUOTA" "$BUCKET"
  exit 2
fi

exit 0
