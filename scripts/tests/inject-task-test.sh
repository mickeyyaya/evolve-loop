#!/usr/bin/env bash
# inject-task-test.sh — CLI validation tests for scripts/utility/inject-task.sh (v9.5.0+).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="$REPO_ROOT/scripts/utility/inject-task.sh"
SCRATCH=$(mktemp -d)

PASS=0; FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

cleanup() { rm -rf "$SCRATCH"; }
trap cleanup EXIT

make_project() {
    local root="$SCRATCH/proj-$RANDOM"
    mkdir -p "$root/.evolve"
    printf '{"carryoverTodos":[],"instinctSummary":[],"failedApproaches":[]}\n' \
        > "$root/.evolve/state.json"
    echo "$root"
}

# --- Test 1: happy path -------------------------------------------------------
header "Test 1: happy path — HIGH priority, action set"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --action "test task one" >/dev/null
count=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | wc -l | tr -d ' ')
[ "$count" -eq 1 ] && pass "one inbox file created" || fail "expected 1 inbox file, got $count"
f=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | head -1)
[ -n "$f" ] && jq empty "$f" 2>/dev/null && pass "inbox file is valid JSON" || fail "inbox file malformed"
act=$(jq -r '.action' "$f" 2>/dev/null)
[ "$act" = "test task one" ] && pass "action field correct" || fail "wrong action: $act"
pri=$(jq -r '.priority' "$f" 2>/dev/null)
[ "$pri" = "HIGH" ] && pass "priority field HIGH" || fail "wrong priority: $pri"

# --- Test 2: priority case normalization -------------------------------------
header "Test 2: priority case normalization (lowercase → upper)"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority medium --action "case test" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | head -1)
pri=$(jq -r '.priority' "$f" 2>/dev/null)
[ "$pri" = "MEDIUM" ] && pass "normalized to MEDIUM" || fail "wrong priority: $pri"

# --- Test 3: validation rejects invalid priority ----------------------------
header "Test 3: invalid priority → exit 10"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority INVALID --action "x" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 10 ] && pass "exit 10 on bad priority" || fail "expected exit 10, got $rc"

# --- Test 4: validation rejects empty action --------------------------------
header "Test 4: empty action → exit 10"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority LOW --action "" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 10 ] && pass "exit 10 on empty action" || fail "expected exit 10, got $rc"

# --- Test 5: weight out of range → exit 10 ----------------------------------
header "Test 5: weight out of range → exit 10"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --weight 1.5 --action "x" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 10 ] && pass "exit 10 on weight > 1.0" || fail "expected exit 10, got $rc"
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --weight -0.1 --action "x" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 10 ] && pass "exit 10 on weight < 0.0" || fail "expected exit 10, got $rc"

# --- Test 6: valid weight stored as float ------------------------------------
header "Test 6: valid weight stored correctly"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --weight 0.85 --action "weighted" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | head -1)
w=$(jq -r '.weight' "$f" 2>/dev/null)
[ "$w" = "0.85" ] && pass "weight 0.85 stored" || fail "wrong weight: $w"

# --- Test 7: absent weight stored as null ------------------------------------
header "Test 7: absent weight → null"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority LOW --action "no weight" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | head -1)
w=$(jq -r '.weight' "$f" 2>/dev/null)
[ "$w" = "null" ] && pass "absent weight is null" || fail "expected null, got '$w'"

# --- Test 8: dry-run does not write file ------------------------------------
header "Test 8: --dry-run emits JSON without writing to disk"
PROJ=$(make_project)
out=$(EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --action "dry test" --dry-run 2>/dev/null)
count=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | wc -l | tr -d ' ')
[ "$count" -eq 0 ] && pass "no file written on dry-run" || fail "file should not be written on dry-run"
echo "$out" | jq empty 2>/dev/null && pass "dry-run output is valid JSON" || fail "dry-run output not valid JSON"

# --- Test 9: id collision with state.json → exit 11 --------------------------
header "Test 9: id collision with state.json → exit 11"
PROJ=$(make_project)
printf '{"carryoverTodos":[{"id":"existing-id","action":"old","priority":"LOW","cycles_unpicked":0,"defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"evidence_pointer":"x"}],"instinctSummary":[],"failedApproaches":[]}\n' \
    > "$PROJ/.evolve/state.json"
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --action "conflict" --id "existing-id" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 11 ] && pass "exit 11 on id collision with state.json" || fail "expected exit 11, got $rc"

# --- Test 10: evidence_pointer auto-synthesized ------------------------------
header "Test 10: evidence_pointer auto-synthesized when absent"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority MEDIUM --action "no ep" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | head -1)
ep=$(jq -r '.evidence_pointer' "$f" 2>/dev/null)
echo "$ep" | grep -q "^inbox-injection://" && \
    pass "evidence_pointer auto-synthesized: $ep" || \
    fail "expected inbox-injection:// prefix, got '$ep'"

# --- Test 11: concurrent calls produce unique ids ----------------------------
header "Test 11: concurrent calls produce unique ids"
PROJ=$(make_project)
mkdir -p "$PROJ/.evolve/inbox"
for i in 1 2 3 4 5; do
    EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority MEDIUM --action "concurrent $i" >/dev/null &
done
wait
count=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | wc -l | tr -d ' ')
[ "$count" -eq 5 ] && pass "5 files created concurrently" || fail "expected 5 files, got $count"
unique_ids=$(jq -r '.id' "$PROJ/.evolve/inbox/"*.json | sort -u | wc -l | tr -d ' ')
[ "$unique_ids" -eq 5 ] && pass "5 unique ids" || fail "expected 5 unique ids, got $unique_ids"

# --- Test 12: --injected-by test round-trips into JSON -----------------------
header "Test 12: --injected-by test → injected_by=test in JSON"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority LOW --action "smoke" --injected-by test >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json 2>/dev/null | head -1)
ib=$(jq -r '.injected_by' "$f" 2>/dev/null)
[ "$ib" = "test" ] && pass "--injected-by test round-trips into JSON" || fail "expected test, got '$ib'"

# --- Test 13: --injected-by garbage → exit 10 --------------------------------
header "Test 13: --injected-by garbage → exit 10 (validation)"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority LOW --action "smoke" --injected-by garbage >/dev/null 2>&1; rc=$?
[ "$rc" -eq 10 ] && pass "exit 10 on --injected-by garbage" || fail "expected exit 10, got $rc"

# --- Summary ------------------------------------------------------------------
echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
