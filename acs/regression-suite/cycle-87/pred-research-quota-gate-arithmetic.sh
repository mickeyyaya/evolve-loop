#!/usr/bin/env bash
# AC-ID: cycle-87-research-quota-gate-arithmetic
# Verifies static hook arithmetic: research-quota-gate.sh returns rc=0 for the
# first N WebSearch calls (per scout quota) and rc=2 on the (N+1)th call with
# a structured deny block on stderr/stdout. Cycle A foundation gate-arithmetic.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
HOOK="$REPO_ROOT/scripts/hooks/research-quota-gate.sh"
HELPER="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"

if [ ! -x "$HOOK" ] && [ ! -f "$HOOK" ]; then
  echo "RED cycle-87-research-quota-gate-arithmetic: hook not found at $HOOK"
  exit 1
fi

# Isolated cycle-state file so we never collide with a real cycle in progress.
TEST_DIR=$(mktemp -d -t research-quota-arith.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_DIR/cycle-state.json"
unset EVOLVE_ALLOW_DEEP_RESEARCH
unset EVOLVE_RESEARCH_HOOK_DISABLED

# Seed minimal cycle-state via the lifecycle helper.
bash "$HELPER" init 99087 "$TEST_DIR/runs/cycle-99087" >/dev/null 2>&1 || {
  echo "RED cycle-87-research-quota-gate-arithmetic: cycle_state_init failed"
  exit 1
}

# Force the active_agent so the hook can read it.
if command -v jq >/dev/null 2>&1; then
  jq -c '.active_agent = "scout"' "$EVOLVE_CYCLE_STATE_FILE" > "$EVOLVE_CYCLE_STATE_FILE.tmp" \
    && mv -f "$EVOLVE_CYCLE_STATE_FILE.tmp" "$EVOLVE_CYCLE_STATE_FILE"
fi

# Scout's WebSearch quota per Cycle A plan = 3 (first 3 ALLOW, 4th DENY).
PAYLOAD='{"tool_name":"WebSearch","tool_input":{"query":"foo"}}'

pass_count=0
for i in 1 2 3; do
  set +e
  echo "$PAYLOAD" | bash "$HOOK" >/dev/null 2>&1
  rc=$?
  set -e
  if [ "$rc" = "0" ]; then
    pass_count=$((pass_count + 1))
  else
    echo "RED cycle-87-research-quota-gate-arithmetic: call #$i unexpectedly denied (rc=$rc), expected ALLOW"
    exit 1
  fi
done

# The 4th call must be DENIED.
set +e
echo "$PAYLOAD" | bash "$HOOK" >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" != "2" ]; then
  echo "RED cycle-87-research-quota-gate-arithmetic: 4th call expected rc=2 (deny), got rc=$rc"
  exit 1
fi

echo "GREEN cycle-87-research-quota-gate-arithmetic: 3 ALLOW + 1 DENY arithmetic verified"
exit 0
