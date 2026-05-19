#!/usr/bin/env bash
# AC-ID: cycle-87-research-quota-deep-flag
# Verifies EVOLVE_ALLOW_DEEP_RESEARCH=1 short-circuits the deny (rc=0 on an
# over-quota call) AND that the deep flag is recorded somewhere observable
# (cycle-state.json:research_usage.deep_overrides or guards.log line).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
HOOK="$REPO_ROOT/scripts/hooks/research-quota-gate.sh"
HELPER="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"

if [ ! -f "$HOOK" ]; then
  echo "RED cycle-87-research-quota-deep-flag: hook not found at $HOOK"
  exit 1
fi

TEST_DIR=$(mktemp -d -t research-quota-deep.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_DIR/cycle-state.json"
export EVOLVE_GUARDS_LOG="$TEST_DIR/guards.log"
unset EVOLVE_RESEARCH_HOOK_DISABLED

bash "$HELPER" init 99087 "$TEST_DIR/runs/cycle-99087" >/dev/null 2>&1
jq -c '.active_agent = "scout"' "$EVOLVE_CYCLE_STATE_FILE" > "$EVOLVE_CYCLE_STATE_FILE.tmp" \
  && mv -f "$EVOLVE_CYCLE_STATE_FILE.tmp" "$EVOLVE_CYCLE_STATE_FILE"

PAYLOAD='{"tool_name":"WebSearch","tool_input":{"query":"foo"}}'

# Burn the quota (scout default = 3).
unset EVOLVE_ALLOW_DEEP_RESEARCH
for i in 1 2 3; do
  echo "$PAYLOAD" | bash "$HOOK" >/dev/null 2>&1 || true
done

# Without override, 4th call must deny.
set +e
echo "$PAYLOAD" | bash "$HOOK" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" != "2" ]; then
  echo "RED cycle-87-research-quota-deep-flag: precondition failed — over-quota call did not deny (rc=$rc)"
  exit 1
fi

# WITH override, the same over-quota call must succeed (rc=0).
set +e
EVOLVE_ALLOW_DEEP_RESEARCH=1 echo "$PAYLOAD" | EVOLVE_ALLOW_DEEP_RESEARCH=1 bash "$HOOK" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" != "0" ]; then
  echo "RED cycle-87-research-quota-deep-flag: EVOLVE_ALLOW_DEEP_RESEARCH=1 did not lift cap (rc=$rc)"
  exit 1
fi

# The deep flag MUST be recorded. Check two observable surfaces:
#   1. cycle-state.json:research_usage.deep_overrides (preferred)
#   2. guards.log line containing "deep" (fallback observability)
deep_in_state=$(jq -r '
    (.research_usage // {}) as $r
    | (
        ($r.deep_overrides // 0)
        + ((($r.scout // {}) | .deep_overrides // 0))
      )
  ' "$EVOLVE_CYCLE_STATE_FILE" 2>/dev/null)

deep_in_log=0
if [ -f "$EVOLVE_GUARDS_LOG" ] && grep -qi 'deep' "$EVOLVE_GUARDS_LOG" 2>/dev/null; then
  deep_in_log=1
fi
# Also tolerate the default guards.log location used by sibling guards.
if [ "$deep_in_log" = "0" ] && [ -f "$REPO_ROOT/.evolve/guards.log" ] \
    && grep -qi 'deep' "$REPO_ROOT/.evolve/guards.log" 2>/dev/null; then
  deep_in_log=1
fi

if [ "${deep_in_state:-0}" -ge 1 ] 2>/dev/null || [ "$deep_in_log" = "1" ]; then
  echo "GREEN cycle-87-research-quota-deep-flag: override lifted cap and deep flag recorded (state=$deep_in_state log=$deep_in_log)"
  exit 0
fi

echo "RED cycle-87-research-quota-deep-flag: override allowed call but deep flag not recorded in cycle-state or guards.log"
exit 1
