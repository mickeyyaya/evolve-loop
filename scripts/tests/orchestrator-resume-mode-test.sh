#!/usr/bin/env bash
# v9.1.0 Cycle 5: orchestrator resume-mode persona + clear-checkpoint op.
#
# Verifies:
#   1. agents/evolve-orchestrator.md has a "## Resume Mode" section
#      documenting EVOLVE_RESUME_MODE / EVOLVE_RESUME_PHASE /
#      EVOLVE_RESUME_COMPLETED_PHASES env vars and the resume protocol.
#   2. cycle-state.sh `clear-checkpoint` operation works correctly.
#   3. The orchestrator profile allowlists the resume-mode cycle-state ops.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
ORCHESTRATOR_MD="$PROJECT_ROOT/agents/evolve-orchestrator.md"
CYCLE_STATE_HELPER="$PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"
ORCH_PROFILE="$PROJECT_ROOT/.evolve/profiles/orchestrator.json"

PASS=0
FAIL=0

expect() {
    local label="$1" actual="$2" expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s (expected=%s actual=%s)\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_match() {
    local label="$1" actual="$2" pattern="$3"
    if [[ "$actual" =~ $pattern ]]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n" "$label" "$pattern" >&2
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test 1: orchestrator persona has Resume Mode section ==="
src=$(cat "$ORCHESTRATOR_MD")
expect_match "## Resume Mode header" "$src" "## Resume Mode"
expect_match "documents EVOLVE_RESUME_MODE" "$src" "EVOLVE_RESUME_MODE"
expect_match "documents EVOLVE_RESUME_PHASE" "$src" "EVOLVE_RESUME_PHASE"
expect_match "documents EVOLVE_RESUME_COMPLETED_PHASES" "$src" "EVOLVE_RESUME_COMPLETED_PHASES"
expect_match "explains 3 pause causes" "$src" "quota-likely.*batch-cap-near.*operator-requested"
expect_match "Resume protocol step list" "$src" "Resume protocol"
expect_match "describes clear-checkpoint step" "$src" "clear-checkpoint"
expect_match "warns no re-running completed phases" "$src" "Do not re-run completed phases"
expect_match "explains checkpoint on intentional pause" "$src" "EVOLVE_CHECKPOINT_REQUEST=1"
expect_match "Verdict CHECKPOINT-PAUSED" "$src" "CHECKPOINT-PAUSED"

echo
echo "=== Test 2: clear-checkpoint operation works ==="
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
export EVOLVE_PROJECT_ROOT="$TMP"
mkdir -p "$TMP/.evolve/runs/cycle-77"
bash "$CYCLE_STATE_HELPER" init 77 ".evolve/runs/cycle-77" >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" advance build builder >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" checkpoint operator-requested >/dev/null 2>&1

# Verify checkpoint exists
bash "$CYCLE_STATE_HELPER" is-checkpointed >/dev/null 2>&1
rc=$?
expect "checkpoint set before clear" "$rc" "0"

# Clear it
out=$(bash "$CYCLE_STATE_HELPER" clear-checkpoint 2>&1)
rc=$?
expect "clear-checkpoint exit code" "$rc" "0"
expect_match "clear-checkpoint emits log" "$out" "checkpoint block cleared"

# Verify it's gone
bash "$CYCLE_STATE_HELPER" is-checkpointed >/dev/null 2>&1
rc=$?
expect "checkpoint cleared (is-checkpointed returns 1)" "$rc" "1"

# Verify cycle-state.json still has other fields (not wholesale-deleted)
STATE_FILE="$TMP/.evolve/cycle-state.json"
phase=$(jq -r '.phase // empty' "$STATE_FILE")
expect "phase preserved after clear-checkpoint" "$phase" "build"
cycle=$(jq -r '.cycle_id // empty' "$STATE_FILE")
expect "cycle_id preserved after clear-checkpoint" "$cycle" "77"

echo
echo "=== Test 3: clear-checkpoint registered in CLI dispatcher ==="
help=$(bash "$CYCLE_STATE_HELPER" 2>&1 || true)
expect_match "usage lists clear-checkpoint" "$help" "clear-checkpoint"

echo
echo "=== Test 4: orchestrator profile allowlists resume-mode ops ==="
profile_src=$(cat "$ORCH_PROFILE")
expect_match "allowlists is-checkpointed" "$profile_src" "cycle-state.sh is-checkpointed"
expect_match "allowlists resume-phase" "$profile_src" "cycle-state.sh resume-phase"
expect_match "allowlists checkpoint" "$profile_src" "cycle-state.sh checkpoint:"
expect_match "allowlists clear-checkpoint" "$profile_src" "cycle-state.sh clear-checkpoint"

echo
echo "=== Test 5: clear-checkpoint is idempotent (safe to call twice) ==="
# Re-init and clear twice — second call should not error.
bash "$CYCLE_STATE_HELPER" clear >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" init 78 ".evolve/runs/cycle-78" >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" advance build builder >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" checkpoint operator-requested >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" clear-checkpoint >/dev/null 2>&1
bash "$CYCLE_STATE_HELPER" clear-checkpoint >/dev/null 2>&1
rc=$?
expect "second clear-checkpoint succeeds" "$rc" "0"

echo
echo "=== Test 6: cycle-state.sh syntax clean ==="
bash -n "$CYCLE_STATE_HELPER" 2>&1 && expect "cycle-state.sh syntax" "ok" "ok" \
    || expect "cycle-state.sh syntax" "FAIL" "ok"

echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    exit 1
fi
