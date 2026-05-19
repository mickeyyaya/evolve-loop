#!/usr/bin/env bash
# AC-ID: cycle-87-research-quota-hook-disabled
# Verifies EVOLVE_RESEARCH_HOOK_DISABLED=1 makes the hook a telemetry-only no-op:
#   - rc=0 on every invocation regardless of usage (no deny ever)
#   - cycle-state research_usage counters still increment (so observability
#     remains visible to the operator even when enforcement is off)
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
HOOK="$REPO_ROOT/scripts/hooks/research-quota-gate.sh"
HELPER="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"

if [ ! -f "$HOOK" ]; then
  echo "RED cycle-87-research-quota-hook-disabled: hook not found at $HOOK"
  exit 1
fi

TEST_DIR=$(mktemp -d -t research-quota-disabled.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_DIR/cycle-state.json"

bash "$HELPER" init 99087 "$TEST_DIR/runs/cycle-99087" >/dev/null 2>&1
jq -c '.active_agent = "scout"' "$EVOLVE_CYCLE_STATE_FILE" > "$EVOLVE_CYCLE_STATE_FILE.tmp" \
  && mv -f "$EVOLVE_CYCLE_STATE_FILE.tmp" "$EVOLVE_CYCLE_STATE_FILE"

PAYLOAD='{"tool_name":"WebSearch","tool_input":{"query":"foo"}}'

# Issue 5 invocations with the hook disabled — well past scout's default quota.
# All five must succeed (rc=0).
fail=0
for i in 1 2 3 4 5; do
  set +e
  EVOLVE_RESEARCH_HOOK_DISABLED=1 \
    bash -c 'echo "$1" | EVOLVE_RESEARCH_HOOK_DISABLED=1 EVOLVE_CYCLE_STATE_FILE="$2" bash "$3" >/dev/null 2>&1' \
    _ "$PAYLOAD" "$EVOLVE_CYCLE_STATE_FILE" "$HOOK"
  rc=$?
  set -e
  if [ "$rc" != "0" ]; then
    echo "RED cycle-87-research-quota-hook-disabled: call #$i denied with rc=$rc (expected ALLOW)"
    exit 1
  fi
done

# Now check that counters DID increment despite the no-op. We expect at least
# one of the standard counter shapes to be present and >= 5.
count=$(jq -r '
    (.research_usage // {}) as $r
    | (
        ($r.web_search // ($r.scout // {} | .web_search // 0))
      ) // 0
  ' "$EVOLVE_CYCLE_STATE_FILE" 2>/dev/null)

# bash 3.2: numeric compare with arithmetic context
if [ -z "$count" ] || [ "$count" = "null" ]; then count=0; fi
if [ "$count" -lt 5 ] 2>/dev/null; then
  echo "RED cycle-87-research-quota-hook-disabled: telemetry counter expected >=5, got '$count'"
  jq . "$EVOLVE_CYCLE_STATE_FILE" >&2 2>/dev/null || true
  exit 1
fi

echo "GREEN cycle-87-research-quota-hook-disabled: 5/5 ALLOW + counter=$count (telemetry preserved)"
exit 0
