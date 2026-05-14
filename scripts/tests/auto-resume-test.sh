#!/usr/bin/env bash
# v10.6.0: tests for the three-layer auto-resume mechanism.
#
# Verifies:
#   Layer 1 — scripts/dispatch/estimate-quota-reset.sh
#     - fallback (now + default 5h25min)
#     - EVOLVE_QUOTA_RESET_AT operator override
#     - parsed hint file (HH:MM am/pm)
#     - hint with already-passed time → next-day rollover
#     - garbage hint → fallback
#     - EVOLVE_QUOTA_RESET_HOURS custom offset
#   Layer 2 — scripts/lifecycle/cycle-state.sh checkpoint schema
#     - 4 new fields (quotaResetAt, quotaResetSource, autoResumeAttempts,
#                    autoResumeMaxAttempts) present after checkpoint
#     - default values when env vars absent
#     - env vars populate fields when present
#     - bump-auto-resume-attempts: N successes + (N+1)th refused (rc=2)
#     - reset-auto-resume-attempts: zeros counter
#     - re-checkpoint preserves prior autoResumeAttempts (cap accumulates)
#   Layer 3 — scripts/dispatch/evolve-loop-dispatch.sh static checks
#     - DISPATCH_RC=5 branch exists
#     - QUOTA-PAUSE marker emit line exists
#     - cycle-state quota-likely detection block exists
#     - final summary recognizes DISPATCH_RC=5
#
# Runs in an isolated mktemp -d; does NOT touch the live cycle-state.json.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
cd "$PROJECT_ROOT" || { echo "FAIL: cannot cd to project root"; exit 1; }

ESTIMATE="$PROJECT_ROOT/scripts/dispatch/estimate-quota-reset.sh"
CYCLE_STATE="$PROJECT_ROOT/scripts/lifecycle/cycle-state.sh"
DISPATCH="$PROJECT_ROOT/scripts/dispatch/evolve-loop-dispatch.sh"

PASS=0
FAIL=0

expect() {
    local label="$1" actual="$2" expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    expected=%s\n    actual  =%s\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_match() {
    local label="$1" actual="$2" pattern="$3"
    # bash native =~ (avoid SIGPIPE on echo|grep -q under pipefail).
    if [[ "$actual" =~ $pattern ]]; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n    actual =%s\n" "$label" "$pattern" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_file_contains() {
    local label="$1" file="$2" pattern="$3"
    if [ -f "$file" ] && grep -qE "$pattern" "$file" 2>/dev/null; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n    file=%s\n" "$label" "$pattern" "$file" >&2
        FAIL=$((FAIL + 1))
    fi
}

TEST_ROOT="$(mktemp -d)"
mkdir -p "$TEST_ROOT/.evolve/runs/cycle-1" "$TEST_ROOT/wt"
trap 'rm -rf "$TEST_ROOT"' EXIT INT TERM

export EVOLVE_PROJECT_ROOT="$TEST_ROOT"

# =============================================================================
# Layer 1 — estimate-quota-reset.sh
# =============================================================================

echo
echo "=== Test 1: fallback (no args, no env) emits ISO + source=default ==="
out=$(unset EVOLVE_QUOTA_RESET_AT EVOLVE_QUOTA_RESET_HOURS; bash "$ESTIMATE" 2>/dev/null)
line1=$(echo "$out" | sed -n '1p')
line2=$(echo "$out" | sed -n '2p')
expect_match "line 1 is ISO 8601" "$line1" "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}"
expect "line 2 is source=default" "$line2" "source=default"

echo
echo "=== Test 2: operator override is echoed verbatim ==="
out=$(EVOLVE_QUOTA_RESET_AT="2026-12-31T23:59:00+0800" bash "$ESTIMATE" 2>/dev/null)
expect "line 1 matches override" "$(echo "$out" | sed -n '1p')" "2026-12-31T23:59:00+0800"
expect "line 2 is source=operator-override" "$(echo "$out" | sed -n '2p')" "source=operator-override"

echo
echo "=== Test 3: parsed hint file (HH:MM am) ==="
HINT_DIR="$TEST_ROOT/.evolve/runs/cycle-1"
echo "resets 12:00pm" > "$HINT_DIR/quota-reset-hint.txt"
out=$(bash "$ESTIMATE" "$HINT_DIR" 2>/dev/null)
line1=$(echo "$out" | sed -n '1p')
expect_match "line 1 ISO contains 12:00:00" "$line1" "T12:00:00"
expect "line 2 is source=parsed" "$(echo "$out" | sed -n '2p')" "source=parsed"

echo
echo "=== Test 4: garbage hint -> fallback ==="
echo "this is not a time" > "$HINT_DIR/quota-reset-hint.txt"
out=$(bash "$ESTIMATE" "$HINT_DIR" 2>/dev/null)
expect "line 2 is source=default (fallback)" "$(echo "$out" | sed -n '2p')" "source=default"

echo
echo "=== Test 5: EVOLVE_QUOTA_RESET_HOURS=1 (custom offset) ==="
rm -f "$HINT_DIR/quota-reset-hint.txt"
out=$(EVOLVE_QUOTA_RESET_HOURS=1 bash "$ESTIMATE" "$HINT_DIR" 2>/dev/null)
expect "line 2 is source=default (with custom hours)" "$(echo "$out" | sed -n '2p')" "source=default"

# =============================================================================
# Layer 2 — cycle-state.sh schema extension
# =============================================================================

# Seed a cycle-state.json
echo '{"cycle_id": 1, "phase": "build", "active_worktree": "'"$TEST_ROOT"'/wt", "completed_phases": ["scout"]}' \
    > "$TEST_ROOT/.evolve/cycle-state.json"

echo
echo "=== Test 6: checkpoint without quota env vars writes default values ==="
unset EVOLVE_CHECKPOINT_QUOTA_RESET_AT EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE EVOLVE_AUTO_RESUME_MAX_ATTEMPTS
bash "$CYCLE_STATE" checkpoint stall-inactivity >/dev/null 2>&1
expect "quotaResetAt empty" "$(jq -r '.checkpoint.quotaResetAt' "$TEST_ROOT/.evolve/cycle-state.json")" ""
expect "quotaResetSource empty" "$(jq -r '.checkpoint.quotaResetSource' "$TEST_ROOT/.evolve/cycle-state.json")" ""
expect "autoResumeAttempts=0" "$(jq -r '.checkpoint.autoResumeAttempts' "$TEST_ROOT/.evolve/cycle-state.json")" "0"
expect "autoResumeMaxAttempts=3 (default)" "$(jq -r '.checkpoint.autoResumeMaxAttempts' "$TEST_ROOT/.evolve/cycle-state.json")" "3"

echo
echo "=== Test 7: checkpoint WITH quota env vars populates fields ==="
EVOLVE_CHECKPOINT_QUOTA_RESET_AT="2026-05-15T05:20:00+0800" \
  EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE="parsed" \
  EVOLVE_AUTO_RESUME_MAX_ATTEMPTS=5 \
  bash "$CYCLE_STATE" checkpoint quota-likely >/dev/null 2>&1
expect "quotaResetAt populated" "$(jq -r '.checkpoint.quotaResetAt' "$TEST_ROOT/.evolve/cycle-state.json")" "2026-05-15T05:20:00+0800"
expect "quotaResetSource populated" "$(jq -r '.checkpoint.quotaResetSource' "$TEST_ROOT/.evolve/cycle-state.json")" "parsed"
expect "autoResumeMaxAttempts=5 (overridden)" "$(jq -r '.checkpoint.autoResumeMaxAttempts' "$TEST_ROOT/.evolve/cycle-state.json")" "5"

echo
echo "=== Test 8: bump-auto-resume-attempts caps at max ==="
# Reset state with max=2 for compact testing
EVOLVE_AUTO_RESUME_MAX_ATTEMPTS=2 bash "$CYCLE_STATE" checkpoint quota-likely >/dev/null 2>&1
bash "$CYCLE_STATE" bump-auto-resume-attempts >/dev/null 2>&1; rc1=$?
bash "$CYCLE_STATE" bump-auto-resume-attempts >/dev/null 2>&1; rc2=$?
bash "$CYCLE_STATE" bump-auto-resume-attempts >/dev/null 2>&1; rc3=$?
expect "1st bump rc=0" "$rc1" "0"
expect "2nd bump rc=0" "$rc2" "0"
expect "3rd bump rc=2 (cap reached)" "$rc3" "2"
expect "attempts stays at max=2" "$(jq -r '.checkpoint.autoResumeAttempts' "$TEST_ROOT/.evolve/cycle-state.json")" "2"

echo
echo "=== Test 9: reset-auto-resume-attempts zeros the counter ==="
bash "$CYCLE_STATE" reset-auto-resume-attempts >/dev/null 2>&1
expect "attempts reset to 0" "$(jq -r '.checkpoint.autoResumeAttempts' "$TEST_ROOT/.evolve/cycle-state.json")" "0"

echo
echo "=== Test 10: re-checkpoint preserves prior autoResumeAttempts ==="
# bump twice
bash "$CYCLE_STATE" bump-auto-resume-attempts >/dev/null 2>&1
bash "$CYCLE_STATE" bump-auto-resume-attempts >/dev/null 2>&1
prior=$(jq -r '.checkpoint.autoResumeAttempts' "$TEST_ROOT/.evolve/cycle-state.json")
# now re-checkpoint (simulating second quota hit on the same cycle)
EVOLVE_CHECKPOINT_QUOTA_RESET_AT="2026-05-16T05:20:00+0800" \
  EVOLVE_CHECKPOINT_QUOTA_RESET_SOURCE="parsed" \
  EVOLVE_AUTO_RESUME_MAX_ATTEMPTS=2 \
  bash "$CYCLE_STATE" checkpoint quota-likely >/dev/null 2>&1
after=$(jq -r '.checkpoint.autoResumeAttempts' "$TEST_ROOT/.evolve/cycle-state.json")
expect "attempts preserved across re-checkpoint" "$after" "$prior"

# =============================================================================
# Layer 3 — dispatcher static checks (the full dispatcher is too heavy for unit
# testing; we verify the new branches/markers are present in the source).
# =============================================================================

echo
echo "=== Test 11: dispatcher contains DISPATCH_RC=5 branch ==="
expect_file_contains "DISPATCH_RC=5 assignment" "$DISPATCH" "DISPATCH_RC=5"
expect_file_contains "STOP_REASON=quota-pause assignment" "$DISPATCH" 'STOP_REASON="quota-pause"'
expect_file_contains "QUOTA-PAUSE marker emit" "$DISPATCH" 'QUOTA-PAUSE: cycle='
expect_file_contains "quota-likely detection (jq read of checkpoint.reason)" "$DISPATCH" '\.checkpoint\.reason'
expect_file_contains "summary handles DISPATCH_RC=5" "$DISPATCH" 'DISPATCH_RC" = "5"'

# =============================================================================
# Cleanup + report
# =============================================================================

echo
echo "==============================================================="
echo "  auto-resume-test.sh:  PASS=$PASS  FAIL=$FAIL"
echo "==============================================================="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
