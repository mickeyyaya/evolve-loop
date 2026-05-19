#!/usr/bin/env bash
# AC-ID: cycle-87-run-cycle-resets-research-usage
# Verifies that run-cycle.sh resets research_usage to zeros at cycle start.
# We do not run a full cycle — instead we exercise the cycle-state subcommands
# (research-usage-incr / research-usage-reset) that run-cycle.sh must call,
# AND we confirm run-cycle.sh actually contains the reset wiring.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
HELPER="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"
RUN_CYCLE="$REPO_ROOT/scripts/dispatch/run-cycle.sh"

# Part 1: cycle-state.sh exposes research-usage-incr / research-usage-reset.
for sub in 'research-usage-incr' 'research-usage-reset'; do
  if ! grep -qE "(research_usage_${sub##*-}|\"$sub\"|research-usage-${sub##*-})" "$HELPER" 2>/dev/null; then
    echo "RED cycle-87-run-cycle-resets-research-usage: cycle-state.sh missing '$sub' subcommand wiring"
    exit 1
  fi
done

# Part 2: run-cycle.sh invokes research-usage-reset at cycle start.
if [ ! -f "$RUN_CYCLE" ]; then
  echo "RED cycle-87-run-cycle-resets-research-usage: run-cycle.sh not found at $RUN_CYCLE"
  exit 1
fi
if ! grep -qE 'research-usage-reset|research_usage_reset' "$RUN_CYCLE"; then
  echo "RED cycle-87-run-cycle-resets-research-usage: run-cycle.sh does not call research-usage-reset"
  exit 1
fi

# Part 3: functional check — incr produces nonzero, reset zeros it.
TEST_DIR=$(mktemp -d -t research-reset.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_DIR/cycle-state.json"

bash "$HELPER" init 99087 "$TEST_DIR/runs/cycle-99087" >/dev/null 2>&1 || {
  echo "RED cycle-87-run-cycle-resets-research-usage: cycle_state_init failed"
  exit 1
}

# Increment a counter twice via the new subcommand.
set +e
bash "$HELPER" research-usage-incr scout web_search >/dev/null 2>&1
rc1=$?
bash "$HELPER" research-usage-incr scout web_search >/dev/null 2>&1
rc2=$?
set -e
if [ "$rc1" != "0" ] || [ "$rc2" != "0" ]; then
  echo "RED cycle-87-run-cycle-resets-research-usage: research-usage-incr failed (rc1=$rc1 rc2=$rc2)"
  exit 1
fi

after_incr=$(jq -r '
    (.research_usage // {}) as $r
    | (($r.scout // {}) | .web_search // ($r.web_search // 0)) // 0
  ' "$EVOLVE_CYCLE_STATE_FILE" 2>/dev/null)
if [ "${after_incr:-0}" -lt 2 ] 2>/dev/null; then
  echo "RED cycle-87-run-cycle-resets-research-usage: counter after 2 increments = $after_incr (expected >=2)"
  exit 1
fi

# Now reset.
set +e
bash "$HELPER" research-usage-reset >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" != "0" ]; then
  echo "RED cycle-87-run-cycle-resets-research-usage: research-usage-reset failed rc=$rc"
  exit 1
fi

after_reset=$(jq -r '
    (.research_usage // {}) as $r
    | (($r.scout // {}) | .web_search // ($r.web_search // 0)) // 0
  ' "$EVOLVE_CYCLE_STATE_FILE" 2>/dev/null)
if [ "${after_reset:-9}" != "0" ]; then
  echo "RED cycle-87-run-cycle-resets-research-usage: counter after reset = $after_reset (expected 0)"
  jq . "$EVOLVE_CYCLE_STATE_FILE" >&2 2>/dev/null || true
  exit 1
fi

echo "GREEN cycle-87-run-cycle-resets-research-usage: incr(2)→$after_incr; reset→$after_reset; run-cycle.sh wired"
exit 0
