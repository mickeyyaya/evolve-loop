#!/usr/bin/env bash
#
# merge-lesson-test.sh — Smoke tests for scripts/failure/merge-lesson-into-state.sh.
# Validates the orchestrator's post-retrospective state-merge logic.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HELPER="$REPO_ROOT/scripts/failure/merge-lesson-into-state.sh"
SCRATCH=$(mktemp -d)

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Build a fake repo with .evolve/state.json + .evolve/instincts/lessons/
make_repo() {
    local root="$SCRATCH/repo-$RANDOM"
    mkdir -p "$root/.evolve/instincts/lessons" "$root/scripts"
    echo '{"instinctSummary": [], "failedApproaches": []}' > "$root/.evolve/state.json"
    : > "$root/.evolve/ledger.jsonl"
    cp "$HELPER" "$root/scripts/failure/merge-lesson-into-state.sh"
    chmod +x "$root/scripts/failure/merge-lesson-into-state.sh"
    echo "$root"
}

# Write a sample lesson YAML
write_lesson() {
    local root="$1" id="$2" pattern="$3" error_cat="$4"
    cat > "$root/.evolve/instincts/lessons/${id}-${pattern}.yaml" <<EOF
- id: $id
  pattern: "$pattern"
  description: "Sample failure pattern for testing the merge helper."
  confidence: 0.8
  source: "cycle-99/test"
  type: "failure-lesson"
  category: "episodic"
  failureContext:
    cycle: 99
    task: "test"
    errorCategory: "$error_cat"
    failedStep: "build"
    auditVerdict: "FAIL"
    auditDefects: ["H1"]
  preventiveAction: "Test action."
  relatedInstincts: []
  contradicts: []
EOF
}

# Write a handoff JSON
write_handoff() {
    local root="$1" cycle="$2" lesson_ids_json="$3" verdict="${4:-FAIL}" systemic="${5:-false}"
    mkdir -p "$root/.evolve/runs/cycle-$cycle"
    cat > "$root/.evolve/runs/cycle-$cycle/handoff-retrospective.json" <<EOF
{
  "cycle": $cycle,
  "auditVerdict": "$verdict",
  "lessonIds": $lesson_ids_json,
  "errorCategory": "reasoning",
  "failedStep": "build",
  "systemic": $systemic,
  "contradictedInstincts": [],
  "preventiveActionCount": 1
}
EOF
}

# --- Test 1: no handoff JSON → no-op ----------------------------------------
header "Test 1: missing handoff is a no-op (PASS cycle)"
ROOT=$(make_repo)
mkdir -p "$ROOT/.evolve/runs/cycle-1"
if EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-1" >/dev/null 2>&1; then
    if [ "$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")" = "0" ]; then
        pass "no-op when handoff missing"
    else
        fail "state.json mutated despite no handoff"
    fi
else
    fail "merge helper returned non-zero on missing handoff"
fi

# --- Test 2: single lesson merged into instinctSummary -----------------------
header "Test 2: single lesson appended to instinctSummary"
ROOT=$(make_repo)
write_lesson "$ROOT" "inst-L001" "shell-substring-bypass" "reasoning"
write_handoff "$ROOT" 99 '["inst-L001"]'
if EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-99" >/dev/null 2>&1; then
    LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
    if [ "$LEN" = "1" ]; then
        ID=$(jq -r '.instinctSummary[0].id' "$ROOT/.evolve/state.json")
        TYPE=$(jq -r '.instinctSummary[0].type' "$ROOT/.evolve/state.json")
        if [ "$ID" = "inst-L001" ] && [ "$TYPE" = "failure-lesson" ]; then
            pass "lesson merged into instinctSummary with correct type"
        else
            fail "wrong id/type: id=$ID type=$TYPE"
        fi
    else
        fail "instinctSummary length=$LEN, expected 1"
    fi
else
    fail "merge returned non-zero"
fi

# --- Test 3: failedApproaches gets one entry per cycle -----------------------
header "Test 3: failedApproaches gains one entry"
LEN=$(jq -r '.failedApproaches | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "1" ] && pass "failedApproaches length=1" || fail "failedApproaches length=$LEN"

# --- Test 4: lesson YAML missing → integrity exit 2 -------------------------
header "Test 4: missing lesson YAML triggers exit 2"
ROOT=$(make_repo)
write_handoff "$ROOT" 99 '["inst-L999"]'   # YAML never written
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-99" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" = "2" ] && pass "exit 2 on missing lesson YAML" || fail "wrong exit: $RC (expected 2)"

# --- Test 5: systemic flag writes ledger event ------------------------------
header "Test 5: systemic flag writes SYSTEMIC_FAILURE ledger event"
ROOT=$(make_repo)
write_lesson "$ROOT" "inst-L002" "recurring-issue" "context"
write_handoff "$ROOT" 100 '["inst-L002"]' "FAIL" "true"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-100" >/dev/null 2>&1
if grep -q '"kind":"SYSTEMIC_FAILURE"' "$ROOT/.evolve/ledger.jsonl"; then
    pass "SYSTEMIC_FAILURE event written"
else
    fail "no SYSTEMIC_FAILURE event in ledger"
fi

# --- Test 6: malformed handoff JSON → fail 1 --------------------------------
header "Test 6: malformed handoff returns 1"
ROOT=$(make_repo)
mkdir -p "$ROOT/.evolve/runs/cycle-99"
echo "not json" > "$ROOT/.evolve/runs/cycle-99/handoff-retrospective.json"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-99" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" = "1" ] && pass "exit 1 on malformed handoff" || fail "wrong exit: $RC"

# --- Test 7: two lessons get two instinctSummary entries --------------------
header "Test 7: two lessons → two instinctSummary entries"
ROOT=$(make_repo)
write_lesson "$ROOT" "inst-L010" "pattern-a" "reasoning"
write_lesson "$ROOT" "inst-L011" "pattern-b" "context"
write_handoff "$ROOT" 101 '["inst-L010","inst-L011"]'
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-101" >/dev/null 2>&1
LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "2" ] && pass "two lessons merged" || fail "expected 2 entries, got $LEN"

# --- Test 8 (v8.56.0): instinctSummary capped at N=5 most-recent ------------
header "Test 8 (v8.56.0): instinctSummary capped at N=5; older archived"
ROOT=$(make_repo)
# Pre-populate state.json with 5 existing entries — adding 1 more should evict oldest.
jq '.instinctSummary = [
    {id:"inst-L100", pattern:"old-1", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L101", pattern:"old-2", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L102", pattern:"old-3", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L103", pattern:"old-4", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L104", pattern:"old-5", confidence:0.5, type:"failure-lesson", errorCategory:"context"}
]' "$ROOT/.evolve/state.json" > "$ROOT/.evolve/state.json.tmp" && \
    mv "$ROOT/.evolve/state.json.tmp" "$ROOT/.evolve/state.json"
write_lesson "$ROOT" "inst-L200" "new-pattern" "reasoning"
write_handoff "$ROOT" 200 '["inst-L200"]'
EVOLVE_INSTINCT_SUMMARY_CAP=5 EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-200" >/dev/null 2>&1
LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
if [ "$LEN" = "5" ]; then
    pass "instinctSummary capped at 5 entries (was 5+1, now 5)"
else
    fail "expected 5 entries after cap, got $LEN"
fi
# The newly added inst-L200 must be present; the oldest (inst-L100) must be evicted.
HAS_NEW=$(jq -r '.instinctSummary[] | select(.id=="inst-L200") | .id' "$ROOT/.evolve/state.json")
HAS_OLD=$(jq -r '.instinctSummary[] | select(.id=="inst-L100") | .id' "$ROOT/.evolve/state.json")
[ "$HAS_NEW" = "inst-L200" ] && pass "newest entry retained" || fail "newest entry not retained"
[ -z "$HAS_OLD" ] && pass "oldest entry evicted (FIFO)" || fail "oldest entry NOT evicted (FIFO violated)"
# Archive file must exist with the evicted entry.
ARCHIVE="$ROOT/.evolve/archive/lessons/instinct-summary-archive.jsonl"
if [ -f "$ARCHIVE" ] && grep -q '"id":"inst-L100"' "$ARCHIVE"; then
    pass "evicted entry written to archive"
else
    fail "archive file missing or evicted entry not present: $ARCHIVE"
fi

# --- Test 9 (v8.56.0): carryoverTodos round-trip ----------------------------
header "Test 9 (v8.56.0): carryover-todos.json round-trips into state.json"
ROOT=$(make_repo)
write_lesson "$ROOT" "inst-L300" "carry-pattern" "context"
write_handoff "$ROOT" 300 '["inst-L300"]'
# Simulate the retrospective writing carryover-todos.json
mkdir -p "$ROOT/.evolve/runs/cycle-300"
cat > "$ROOT/.evolve/runs/cycle-300/carryover-todos.json" <<TODOEOF
[
  {"id":"todo-1","action":"Add unit test for shell parser edge cases","priority":"high","evidence_pointer":"audit-report.md#D1"},
  {"id":"todo-2","action":"Document bash 3.2 compat requirement","priority":"medium","evidence_pointer":"build-report.md"}
]
TODOEOF
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-300" >/dev/null 2>&1
TODO_LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json" 2>/dev/null)
if [ "$TODO_LEN" = "2" ]; then
    pass "carryoverTodos has 2 entries"
else
    fail "expected 2 carryoverTodos, got $TODO_LEN"
fi
# Each todo gains defer_count=0 on first write
DC=$(jq -r '.carryoverTodos[0].defer_count // empty' "$ROOT/.evolve/state.json")
[ "$DC" = "0" ] && pass "defer_count initialized to 0" || fail "defer_count missing or wrong: '$DC'"
# Cycle pointer is recorded
CYC=$(jq -r '.carryoverTodos[0].first_seen_cycle // empty' "$ROOT/.evolve/state.json")
[ "$CYC" = "300" ] && pass "first_seen_cycle recorded" || fail "first_seen_cycle wrong: '$CYC'"

# --- Test 10 (v8.56.0): re-deferring same todo increments defer_count -------
header "Test 10 (v8.56.0): re-deferring increments defer_count"
# Run AGAIN with the same todo id — defer_count should bump from 0 → 1
write_handoff "$ROOT" 301 '["inst-L300"]'
mkdir -p "$ROOT/.evolve/runs/cycle-301"
cat > "$ROOT/.evolve/runs/cycle-301/carryover-todos.json" <<TODOEOF
[
  {"id":"todo-1","action":"Add unit test for shell parser edge cases","priority":"high","evidence_pointer":"audit-report.md#D1"}
]
TODOEOF
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-301" >/dev/null 2>&1
DC=$(jq -r '.carryoverTodos[] | select(.id=="todo-1") | .defer_count' "$ROOT/.evolve/state.json")
[ "$DC" = "1" ] && pass "defer_count incremented on re-defer" || fail "defer_count not incremented: '$DC'"

# --- Test 11 (v8.56.0): warn on defer_count >= 3 ----------------------------
header "Test 11 (v8.56.0): defer_count >= 3 emits WARN to stderr"
# Bump defer_count to 2 directly via jq, then run merge once more → should be 3 → WARN
jq '.carryoverTodos = [.carryoverTodos[] | if .id=="todo-1" then .defer_count=2 else . end]' "$ROOT/.evolve/state.json" > "$ROOT/.evolve/state.json.tmp" && \
    mv "$ROOT/.evolve/state.json.tmp" "$ROOT/.evolve/state.json"
write_handoff "$ROOT" 302 '["inst-L300"]'
mkdir -p "$ROOT/.evolve/runs/cycle-302"
cat > "$ROOT/.evolve/runs/cycle-302/carryover-todos.json" <<TODOEOF
[{"id":"todo-1","action":"Add unit test for shell parser edge cases","priority":"high","evidence_pointer":"audit-report.md#D1"}]
TODOEOF
WARN_OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-302" 2>&1 >/dev/null)
if echo "$WARN_OUT" | grep -q "WARN.*defer.*3"; then
    pass "WARN emitted when defer_count reaches 3"
else
    fail "expected WARN about defer_count=3 in stderr; got: $WARN_OUT"
fi

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
