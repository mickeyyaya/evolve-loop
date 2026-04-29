#!/usr/bin/env bash
#
# cycle-state-test.sh — Unit tests for scripts/cycle-state.sh (v8.13.1).
#
# v8.13.7: cycle-state.sh is on the hot path of TWO gates (role-gate and
# phase-gate-precondition), but had only indirect coverage via role-gate-test.sh.
# This suite covers init/get/advance/clear lifecycle, the JSON schema, jq
# fallback paths, and the malformed-input guards.
#
# Tests use EVOLVE_CYCLE_STATE_FILE to redirect to a per-test temp file so the
# real .evolve/cycle-state.json is never touched.
#
# Usage: bash scripts/cycle-state-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/cycle-state.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Per-test fresh state file — EVOLVE_CYCLE_STATE_FILE redirects the script away
# from .evolve/cycle-state.json so we never collide with a real cycle in flight.
fresh_state() {
    local f
    f=$(mktemp -t cycle-state-test.XXXXXX.json)
    rm -f "$f"   # we want the path but no content
    echo "$f"
}

cleanup_files=()
trap 'for f in "${cleanup_files[@]}"; do rm -f "$f"; done' EXIT

# === Test 1: init creates a valid JSON state file =============================
header "Test 1: init writes a parseable JSON file with required fields"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9001 .evolve/runs/cycle-9001 >/dev/null 2>&1
if [ ! -f "$sf" ]; then
    fail_ "state file not created at $sf"
elif ! jq -e . "$sf" >/dev/null 2>&1; then
    fail_ "state file is not valid JSON: $(cat "$sf")"
else
    cycle=$(jq -r '.cycle_id' "$sf")
    phase=$(jq -r '.phase' "$sf")
    ws=$(jq -r '.workspace_path' "$sf")
    if [ "$cycle" = "9001" ] && [ "$phase" = "calibrate" ] && [ "$ws" = ".evolve/runs/cycle-9001" ]; then
        pass "init created valid state with correct fields"
    else
        fail_ "field mismatch: cycle=$cycle phase=$phase ws=$ws"
    fi
fi

# === Test 2: init with default workspace path ================================
header "Test 2: init uses default workspace path when arg2 omitted"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9002 >/dev/null 2>&1
ws=$(jq -r '.workspace_path' "$sf")
if [ "$ws" = ".evolve/runs/cycle-9002" ]; then
    pass "default workspace path derived from cycle_id"
else
    fail_ "expected .evolve/runs/cycle-9002, got $ws"
fi

# === Test 3: get returns field value =========================================
header "Test 3: get phase returns the current phase string"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9003 >/dev/null 2>&1
out=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" get phase)
if [ "$out" = "calibrate" ]; then
    pass "get phase returns 'calibrate' after init"
else
    fail_ "expected 'calibrate', got '$out'"
fi

# === Test 4: get cycle_id returns numeric string =============================
header "Test 4: get cycle_id returns numeric string"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9004 >/dev/null 2>&1
out=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" get cycle_id)
if [ "$out" = "9004" ]; then
    pass "get cycle_id returns '9004'"
else
    fail_ "expected '9004', got '$out'"
fi

# === Test 5: advance updates phase + records previous phase as completed =====
header "Test 5: advance moves phase and records previous in completed_phases"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9005 >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance research >/dev/null 2>&1
new_phase=$(jq -r '.phase' "$sf")
completed=$(jq -r '.completed_phases | join(",")' "$sf")
if [ "$new_phase" = "research" ] && [ "$completed" = "calibrate" ]; then
    pass "phase=research, completed=[calibrate]"
else
    fail_ "phase=$new_phase, completed=$completed"
fi

# === Test 6: advance with agent + worktree records both ======================
header "Test 6: advance build builder /tmp/wt-foo records agent and worktree"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9006 >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance research >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance build builder /tmp/wt-foo >/dev/null 2>&1
agent=$(jq -r '.active_agent' "$sf")
wt=$(jq -r '.active_worktree' "$sf")
if [ "$agent" = "builder" ] && [ "$wt" = "/tmp/wt-foo" ]; then
    pass "active_agent=builder, active_worktree=/tmp/wt-foo"
else
    fail_ "agent=$agent worktree=$wt"
fi

# === Test 7: advance dedupes by checking previous-phase membership ===========
# The dedupe check at cycle-state.sh:118 is `($s.completed_phases | index($cur)) == null`
# where $cur is the PREVIOUS phase. So advancing A→B→A→A produces
# completed=[A, B, A], not [A, B] — the dedupe only catches A→A→A→A patterns.
# This test pins current behavior. If the spec ever requires "advance into a
# phase already recorded does not re-record it", change the jq filter to also
# compare $new_phase against $cur and short-circuit when equal.
header "Test 7: dedupe pins previous-phase index check (current-behavior pin)"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9007 >/dev/null 2>&1
# A→A→A: dedupe catches all three because $cur is in completed_phases by step 2
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance research >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance research >/dev/null 2>&1
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance research >/dev/null 2>&1
# After init: phase=calibrate, completed=[]
# advance research #1: $cur=calibrate, append → completed=[calibrate], phase=research
# advance research #2: $cur=research, append (research not in [calibrate]) → completed=[calibrate, research], phase=research
# advance research #3: $cur=research, NO append (research already in completed) → completed=[calibrate, research], phase=research
got=$(jq -c '.completed_phases' "$sf")
if [ "$got" = '["calibrate","research"]' ]; then
    pass "dedupe pins observed: A→A→A produces [previous, A]"
else
    fail_ "expected [calibrate,research], got $got"
fi

# === Test 8: clear removes the state file ====================================
header "Test 8: clear removes the state file"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9008 >/dev/null 2>&1
[ -f "$sf" ] || { fail_ "init didn't create file"; }
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" clear >/dev/null 2>&1
if [ ! -f "$sf" ]; then
    pass "state file removed by clear"
else
    fail_ "state file still exists after clear"
fi

# === Test 9: clear is idempotent (no error if file missing) ==================
header "Test 9: clear on already-absent file does not error"
sf=$(fresh_state); cleanup_files+=("$sf")
set +e
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" clear >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "0" ]; then
    pass "clear on missing file returns 0"
else
    fail_ "clear on missing file returned rc=$rc"
fi

# === Test 10: exists returns yes/no with correct exit code ===================
header "Test 10: exists returns 'yes' rc=0 when present, 'no' rc=1 when absent"
sf=$(fresh_state); cleanup_files+=("$sf")
set +e
out_no=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" exists 2>&1)
rc_no=$?
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9010 >/dev/null 2>&1
out_yes=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" exists 2>&1)
rc_yes=$?
set -e
if [ "$out_no" = "no" ] && [ "$rc_no" = "1" ] && [ "$out_yes" = "yes" ] && [ "$rc_yes" = "0" ]; then
    pass "exists returns no/rc=1 when absent, yes/rc=0 when present"
else
    fail_ "absent: out=$out_no rc=$rc_no; present: out=$out_yes rc=$rc_yes"
fi

# === Test 11: advance without prior init fails loudly ========================
# Calling advance before init must fail — silently advancing on a missing file
# would mask programmer errors that lead to lost cycle context.
header "Test 11: advance without init fails with clear error"
sf=$(fresh_state); cleanup_files+=("$sf")
set +e
out=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance build builder /tmp/wt 2>&1)
rc=$?
set -e
if [ "$rc" != "0" ] && echo "$out" | grep -q "state file missing"; then
    pass "advance-without-init returns rc=$rc + 'state file missing'"
else
    fail_ "expected rc!=0 with 'state file missing', got rc=$rc out='$out'"
fi

# === Test 12: get on missing file returns rc=1 ===============================
header "Test 12: get on missing state file returns rc=1"
sf=$(fresh_state); cleanup_files+=("$sf")
set +e
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" get phase >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "1" ]; then
    pass "get on missing file returns rc=1"
else
    fail_ "expected rc=1, got rc=$rc"
fi

# === Test 13: dump prints the file contents ==================================
header "Test 13: dump prints state JSON"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9013 >/dev/null 2>&1
out=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" dump)
if echo "$out" | jq -e '.cycle_id == 9013' >/dev/null 2>&1; then
    pass "dump prints valid JSON containing cycle_id"
else
    fail_ "dump output not the expected JSON: $out"
fi

# === Test 14: path prints the configured state file path =====================
header "Test 14: path prints \$EVOLVE_CYCLE_STATE_FILE when set"
sf=$(fresh_state); cleanup_files+=("$sf")
out=$(EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" path)
if [ "$out" = "$sf" ]; then
    pass "path returns the override value"
else
    fail_ "expected $sf, got $out"
fi

# === Test 15: full lifecycle (init → advance × N → clear) ====================
# End-to-end: walk through the canonical phase sequence and verify each step.
header "Test 15: full lifecycle calibrate→research→discover→build→audit→ship→learn"
sf=$(fresh_state); cleanup_files+=("$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" init 9015 >/dev/null 2>&1
for ph in research discover build audit ship learn; do
    EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" advance "$ph" >/dev/null 2>&1
done
final_phase=$(jq -r '.phase' "$sf")
completed_count=$(jq -r '.completed_phases | length' "$sf")
EVOLVE_CYCLE_STATE_FILE="$sf" bash "$SCRIPT" clear >/dev/null 2>&1
if [ "$final_phase" = "learn" ] \
   && [ "$completed_count" = "6" ] \
   && [ ! -f "$sf" ]; then
    pass "lifecycle complete: ended at learn, 6 completed phases, file cleared"
else
    fail_ "phase=$final_phase, completed=$completed_count, file_exists=$([ -f "$sf" ] && echo y || echo n)"
fi

# === Test 16: bad subcommand returns rc=2 ====================================
header "Test 16: unknown subcommand returns rc=2 with usage line"
set +e
out=$(bash "$SCRIPT" not-a-real-command 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -q "usage:"; then
    pass "unknown subcommand: rc=2 + usage hint"
else
    fail_ "expected rc=2 with 'usage:', got rc=$rc out='$out'"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
