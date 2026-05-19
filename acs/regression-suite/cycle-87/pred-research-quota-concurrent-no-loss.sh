#!/usr/bin/env bash
# AC-ID: cycle-87-research-quota-concurrent-no-loss
# Verifies fan-out safety: concurrent invocations of research-quota-gate.sh
# for the same agent produce a final counter equal to the sum of invocations.
# Catches the classic read-modify-write race (atomic mv alone is insufficient;
# Builder must add a file lock — flock or mkdir-based — around the RMW).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
HOOK="$REPO_ROOT/scripts/hooks/research-quota-gate.sh"
HELPER="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"

if [ ! -f "$HOOK" ]; then
  echo "RED cycle-87-research-quota-concurrent-no-loss: hook not found at $HOOK"
  exit 1
fi

TEST_DIR=$(mktemp -d -t research-quota-concurrent.XXXXXX)
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_DIR/cycle-state.json"

# Use the disabled-mode no-op path so all calls are guaranteed ALLOW but still
# increment counters — this isolates the test to *counter atomicity*, not the
# allow/deny decision. Per intent.md acceptance #3, disabled mode must still
# increment.
export EVOLVE_RESEARCH_HOOK_DISABLED=1

bash "$HELPER" init 99087 "$TEST_DIR/runs/cycle-99087" >/dev/null 2>&1
jq -c '.active_agent = "scout"' "$EVOLVE_CYCLE_STATE_FILE" > "$EVOLVE_CYCLE_STATE_FILE.tmp" \
  && mv -f "$EVOLVE_CYCLE_STATE_FILE.tmp" "$EVOLVE_CYCLE_STATE_FILE"

# Fan-out: spawn N concurrent hook invocations. Choose N high enough to surface
# the race on modern hardware; with no lock, lost updates are extremely likely
# in this window. Keep N modest enough that the test completes in <10s.
N=20
PAYLOAD='{"tool_name":"WebSearch","tool_input":{"query":"race"}}'

pids=""
for i in $(seq 1 "$N"); do
  ( echo "$PAYLOAD" | bash "$HOOK" >/dev/null 2>&1 ) &
  pids="$pids $!"
done

# Wait for every child to finish.
wait $pids 2>/dev/null || true

count=$(jq -r '
    (.research_usage // {}) as $r
    | (($r.scout // {}) | .web_search // ($r.web_search // 0)) // 0
  ' "$EVOLVE_CYCLE_STATE_FILE" 2>/dev/null)

if [ -z "$count" ] || [ "$count" = "null" ]; then count=0; fi
if [ "$count" != "$N" ]; then
  echo "RED cycle-87-research-quota-concurrent-no-loss: $count/$N (lost updates — RMW race not locked)"
  jq . "$EVOLVE_CYCLE_STATE_FILE" >&2 2>/dev/null || true
  exit 1
fi

echo "GREEN cycle-87-research-quota-concurrent-no-loss: $count/$N concurrent invocations counted (no lost updates)"
exit 0
