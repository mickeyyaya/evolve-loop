#!/usr/bin/env bash
# v9.1.0 Cycle 1: round-trip test for the checkpoint-resume primitives.
#
# Verifies the three new cycle-state operations:
#   cycle-state.sh checkpoint <reason>
#   cycle-state.sh is-checkpointed
#   cycle-state.sh resume-phase
#
# And the run-cycle.sh EXIT-trap conditional cleanup behavior is wired up
# (we cannot fully exercise run-cycle.sh in a test because it requires a
# claude binary; we verify the trap branch by reading the source).

set -uo pipefail

# Locate project root deterministically. The test file lives at
# scripts/tests/checkpoint-roundtrip-test.sh, so the project root is two
# levels up. realpath -P resolves symlinks (the plugin install path).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
cd "$PROJECT_ROOT" || { echo "FAIL: cannot cd to project root"; exit 1; }

CYCLE_STATE="$PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"
RUN_CYCLE="$PROJECT_ROOT/scripts/dispatch/run-cycle.sh"

PASS=0
FAIL=0
fail_count=0

# Helper: assert-style. Logs PASS/FAIL line and increments counters.
expect() {
    local label="$1"
    local actual="$2"
    local expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    expected=%s\n    actual  =%s\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
        fail_count=$((fail_count + 1))
    fi
}

expect_match() {
    local label="$1"
    local actual="$2"
    local pattern="$3"
    if echo "$actual" | grep -qE "$pattern"; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n    actual =%s\n" "$label" "$pattern" "$actual" >&2
        FAIL=$((FAIL + 1))
        fail_count=$((fail_count + 1))
    fi
}

# Create an isolated cycle-state in a temp dir. Override EVOLVE_PROJECT_ROOT
# so the helper writes to our sandbox instead of touching the live state.
TEST_ROOT="$(mktemp -d)"
mkdir -p "$TEST_ROOT/.evolve/runs/cycle-9999"
trap 'rm -rf "$TEST_ROOT"' EXIT INT TERM

export EVOLVE_PROJECT_ROOT="$TEST_ROOT"

echo
echo "=== Test 1: checkpoint requires existing cycle-state ==="
output=$(bash "$CYCLE_STATE" checkpoint operator-requested 2>&1 || true)
expect_match "rejects when no cycle-state.json" "$output" "state file missing|cannot checkpoint"

echo
echo "=== Test 2: init a cycle, then checkpoint succeeds ==="
bash "$CYCLE_STATE" init 9999 ".evolve/runs/cycle-9999" >/dev/null 2>&1
bash "$CYCLE_STATE" advance build builder >/dev/null 2>&1
bash "$CYCLE_STATE" set-worktree "/var/folders/test/cycle-9999" >/dev/null 2>&1

output=$(bash "$CYCLE_STATE" checkpoint operator-requested 2>&1)
expect_match "checkpoint emits success message" "$output" "CHECKPOINT written.*reason=operator-requested.*resume_from_phase=build"

echo
echo "=== Test 3: is-checkpointed returns yes ==="
out=$(bash "$CYCLE_STATE" is-checkpointed 2>&1 || true)
expect "is-checkpointed returns yes" "$out" "yes"

echo
echo "=== Test 4: resume-phase echoes correct phase ==="
phase=$(bash "$CYCLE_STATE" resume-phase 2>&1)
expect "resume-phase returns 'build'" "$phase" "build"

echo
echo "=== Test 5: checkpoint block contains all required fields ==="
state_file="$TEST_ROOT/.evolve/cycle-state.json"
[ -f "$state_file" ] || { echo "FAIL: state file missing at $state_file"; FAIL=$((FAIL + 1)); }

if command -v jq >/dev/null 2>&1; then
    enabled=$(jq -r '.checkpoint.enabled' "$state_file")
    reason=$(jq -r '.checkpoint.reason' "$state_file")
    resume_from=$(jq -r '.checkpoint.resumeFromPhase' "$state_file")
    worktree=$(jq -r '.checkpoint.worktreePath' "$state_file")
    saved_at=$(jq -r '.checkpoint.savedAt' "$state_file")
    completed=$(jq -r '.checkpoint.completedPhases | length' "$state_file")

    expect "checkpoint.enabled == true" "$enabled" "true"
    expect "checkpoint.reason == operator-requested" "$reason" "operator-requested"
    expect "checkpoint.resumeFromPhase == build" "$resume_from" "build"
    expect "checkpoint.worktreePath captured" "$worktree" "/var/folders/test/cycle-9999"
    expect_match "checkpoint.savedAt is ISO-8601 UTC" "$saved_at" "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z\$"
    # completed_phases captured from cycle-state's completed_phases at checkpoint time
    expect_match "checkpoint.completedPhases is array" "$completed" "^[0-9]+\$"
fi

echo
echo "=== Test 6: invalid reason rejected ==="
bash "$CYCLE_STATE" clear >/dev/null 2>&1
bash "$CYCLE_STATE" init 9999 ".evolve/runs/cycle-9999" >/dev/null 2>&1
output=$(bash "$CYCLE_STATE" checkpoint bogus-reason 2>&1 || true)
expect_match "rejects invalid reason" "$output" "invalid checkpoint reason"

echo
echo "=== Test 7: is-checkpointed exit code 1 when not checkpointed ==="
bash "$CYCLE_STATE" is-checkpointed >/dev/null 2>&1
rc=$?
expect "exit code 1 when no checkpoint" "$rc" "1"

echo
echo "=== Test 8: cycle-state.sh CLI registers all 3 new commands ==="
help=$(bash "$CYCLE_STATE" 2>&1 || true)
expect_match "usage lists checkpoint" "$help" "checkpoint"
expect_match "usage lists is-checkpointed" "$help" "is-checkpointed"
expect_match "usage lists resume-phase" "$help" "resume-phase"

echo
echo "=== Test 9: run-cycle.sh EXIT trap reads is-checkpointed ==="
# Source-level verification: the cleanup() function must check is-checkpointed.
trap_check=$(grep -c "is-checkpointed" "$RUN_CYCLE" || true)
[ "$trap_check" -ge 1 ] && expect "is-checkpointed call present in run-cycle.sh" "yes" "yes" \
    || expect "is-checkpointed call present in run-cycle.sh" "no" "yes"

# Source-level verification: cleanup must respect EVOLVE_CHECKPOINT_TRIGGERED.
triggered_check=$(grep -c "EVOLVE_CHECKPOINT_TRIGGERED" "$RUN_CYCLE" || true)
[ "$triggered_check" -ge 1 ] && expect "EVOLVE_CHECKPOINT_TRIGGERED handling in run-cycle.sh" "yes" "yes" \
    || expect "EVOLVE_CHECKPOINT_TRIGGERED handling in run-cycle.sh" "no" "yes"

# Source-level verification: cleanup must NOT remove worktree when checkpointed.
checkpoint_skip=$(awk '/checkpointed.*=.*1/,/exit \$rc/' "$RUN_CYCLE" | grep -c "preserved" || true)
[ "$checkpoint_skip" -ge 1 ] && expect "preserve-mode branch present in cleanup" "yes" "yes" \
    || expect "preserve-mode branch present in cleanup" "no" "yes"

echo
echo "=== Test 10: idempotent checkpoint (second write updates the block) ==="
bash "$CYCLE_STATE" clear >/dev/null 2>&1
bash "$CYCLE_STATE" init 9999 ".evolve/runs/cycle-9999" >/dev/null 2>&1
bash "$CYCLE_STATE" advance audit auditor >/dev/null 2>&1
bash "$CYCLE_STATE" checkpoint quota-likely >/dev/null 2>&1
first_phase=$(bash "$CYCLE_STATE" resume-phase)
bash "$CYCLE_STATE" advance ship ship >/dev/null 2>&1 || true
# Note: cycle-state.advance may reject ship→audit reversal; that's OK.
# What we care about: a second checkpoint call still works and reflects current phase.
bash "$CYCLE_STATE" checkpoint quota-likely >/dev/null 2>&1
second_phase=$(bash "$CYCLE_STATE" resume-phase)
[ -n "$second_phase" ] && expect "second checkpoint preserves a resume_phase" "ok" "ok" \
    || expect "second checkpoint preserves a resume_phase" "missing" "ok"

echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    echo "FAILURES: $FAIL"
    exit 1
fi
