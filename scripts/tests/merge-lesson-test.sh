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

# --- Test 12 (v8.58.0): lessonFiles fallback — workspace lessons recovered ----
header "Test 12 (v8.58.0): lessonFiles fallback — workspace lessons merged when lessonIds empty"
ROOT=$(make_repo)
mkdir -p "$ROOT/.evolve/runs/cycle-500"
cat > "$ROOT/.evolve/runs/cycle-500/lesson-alpha.yaml" <<YAMLEOF
- id: inst-T12-alpha
  pattern: "test-12-pattern-alpha"
  description: "Test 12 lesson alpha from workspace fallback."
  confidence: 0.75
  type: "failure-lesson"
  failureContext:
    errorCategory: "code-audit-warn"
YAMLEOF
cat > "$ROOT/.evolve/runs/cycle-500/lesson-beta.yaml" <<YAMLEOF
- id: inst-T12-beta
  pattern: "test-12-pattern-beta"
  description: "Test 12 lesson beta from workspace fallback."
  confidence: 0.80
  type: "failure-lesson"
  failureContext:
    errorCategory: "code-audit-warn"
YAMLEOF
cat > "$ROOT/.evolve/runs/cycle-500/handoff-retrospective.json" <<HEOF
{
  "cycle": 500,
  "auditVerdict": "WARN",
  "lessonIds": [],
  "lessonFiles": ["lesson-alpha.yaml", "lesson-beta.yaml"],
  "lessonFilesNote": "role-gate blocked write to canonical dir; lessons in workspace",
  "errorCategory": "code-audit-warn",
  "failedStep": "audit",
  "systemic": false,
  "contradictedInstincts": []
}
HEOF
if EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-500" >/dev/null 2>&1; then
    LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
    [ "$LEN" = "2" ] && pass "lessonFiles fallback: 2 workspace lessons merged into instinctSummary" || fail "expected 2 instinctSummary entries, got $LEN"
    HAS_ALPHA=$(jq -r '.instinctSummary[] | select(.id=="inst-T12-alpha") | .id' "$ROOT/.evolve/state.json")
    [ "$HAS_ALPHA" = "inst-T12-alpha" ] && pass "inst-T12-alpha recovered via lessonFiles fallback" || fail "inst-T12-alpha missing from instinctSummary"
    HAS_BETA=$(jq -r '.instinctSummary[] | select(.id=="inst-T12-beta") | .id' "$ROOT/.evolve/state.json")
    [ "$HAS_BETA" = "inst-T12-beta" ] && pass "inst-T12-beta recovered via lessonFiles fallback" || fail "inst-T12-beta missing from instinctSummary"
    [ -f "$ROOT/.evolve/instincts/lessons/inst-T12-alpha.yaml" ] && pass "lesson YAML copied to canonical LESSONS_DIR" || fail "lesson not copied to canonical LESSONS_DIR"
else
    fail "merge returned non-zero for lessonFiles fallback"
fi

# --- Test 13 (v8.58.0): retrospected flag updated after successful merge ------
header "Test 13 (v8.58.0): retrospected flag set to true after lesson merge"
ROOT=$(make_repo)
# Pre-populate with two failedApproaches entries: cycle 99 (target) and cycle 1 (untouched)
jq '.failedApproaches = [
    {ts:"2026-01-01T00:00:00Z", cycle:99, verdict:"WARN", retrospected:false, auditReportSha256:"abc123"},
    {ts:"2026-01-02T00:00:00Z", cycle:1,  verdict:"WARN", retrospected:false, auditReportSha256:"def456"}
]' "$ROOT/.evolve/state.json" > "$ROOT/.evolve/state.json.tmp" && \
    mv "$ROOT/.evolve/state.json.tmp" "$ROOT/.evolve/state.json"
write_lesson "$ROOT" "inst-T13-retro" "retrospect-flag-test" "reasoning"
write_handoff "$ROOT" 99 '["inst-T13-retro"]' "WARN"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-99" >/dev/null 2>&1
# cycle-99 entry (has explicit retrospected field) must now be true
FALSE_99=$(jq '[.failedApproaches[] | select(.cycle==99 and .retrospected==false)] | length' "$ROOT/.evolve/state.json")
[ "$FALSE_99" = "0" ] && pass "cycle-99 failedApproaches entry marked retrospected=true" || fail "cycle-99 still has $FALSE_99 entry/entries with retrospected=false"
# cycle-1 entry must remain unchanged (different cycle)
FALSE_1=$(jq '[.failedApproaches[] | select(.cycle==1 and .retrospected==false)] | length' "$ROOT/.evolve/state.json")
[ "$FALSE_1" = "1" ] && pass "cycle-1 failedApproaches entry remains retrospected=false (untouched)" || fail "cycle-1 entry was unexpectedly modified"

# --- Test 14 (v8.59.0): instinctCount updated on single lesson append --------
header "Test 14 (v8.59.0): instinctCount=1 after single lesson append to empty state"
ROOT=$(make_repo)
write_lesson "$ROOT" "inst-L014" "test-14-pattern" "reasoning"
write_handoff "$ROOT" 14 '["inst-L014"]'
if EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-14" >/dev/null 2>&1; then
    COUNT=$(jq -r '.instinctCount // 0' "$ROOT/.evolve/state.json")
    LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
    if [ "$COUNT" = "1" ] && [ "$LEN" = "1" ]; then
        pass "instinctCount=1 after single append (Fix A: counter updated)"
    else
        fail "instinctCount=$COUNT instinctSummary.length=$LEN; expected both=1"
    fi
else
    fail "merge returned non-zero"
fi

# --- Test 15 (v8.59.0): self-heal fires when instinctCount drifted ----------
header "Test 15 (v8.59.0): self-heal: instinctCount=0 with instinctSummary.length=3 → heals to 4 after append, WARN logged"
ROOT=$(make_repo)
jq '.instinctSummary = [
    {id:"inst-L015-pre1", pattern:"pre-1", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L015-pre2", pattern:"pre-2", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L015-pre3", pattern:"pre-3", confidence:0.5, type:"failure-lesson", errorCategory:"context"}
] | .instinctCount = 0' "$ROOT/.evolve/state.json" > "$ROOT/.evolve/state.json.tmp" && \
    mv "$ROOT/.evolve/state.json.tmp" "$ROOT/.evolve/state.json"
write_lesson "$ROOT" "inst-L015-new" "test-15-new-pattern" "reasoning"
write_handoff "$ROOT" 15 '["inst-L015-new"]'
STDERR_15=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-15" 2>&1 >/dev/null)
COUNT=$(jq -r '.instinctCount // 0' "$ROOT/.evolve/state.json")
LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
if [ "$COUNT" = "4" ] && [ "$LEN" = "4" ]; then
    pass "instinctCount=4 after self-heal + append (Fix C + Fix A)"
else
    fail "instinctCount=$COUNT instinctSummary.length=$LEN; expected both=4"
fi
if echo "$STDERR_15" | grep -q "WARN \[bookkeeping\]"; then
    pass "WARN [bookkeeping] logged for drifted instinctCount (Fix C)"
else
    fail "expected WARN [bookkeeping] in stderr; got: $STDERR_15"
fi

# --- Test 16 (v8.59.0): dedup guard — same lesson ID merged twice → no count change ---
header "Test 16 (v8.59.0): idempotent re-merge: same lesson ID merged twice → instinctCount=1"
ROOT=$(make_repo)
write_lesson "$ROOT" "inst-L016-dup" "test-16-dup-pattern" "reasoning"
write_handoff "$ROOT" 16 '["inst-L016-dup"]'
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-16" >/dev/null 2>&1
COUNT=$(jq -r '.instinctCount // 0' "$ROOT/.evolve/state.json")
[ "$COUNT" = "1" ] && pass "instinctCount=1 after first merge" || fail "instinctCount=$COUNT after first merge; expected 1"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-16" >/dev/null 2>&1
COUNT=$(jq -r '.instinctCount // 0' "$ROOT/.evolve/state.json")
LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
if [ "$COUNT" = "1" ] && [ "$LEN" = "1" ]; then
    pass "instinctCount=1 and length=1 after second merge (dedup guard + Fix A)"
else
    fail "instinctCount=$COUNT instinctSummary.length=$LEN after second merge; expected both=1"
fi

# --- Test 17 (v8.59.0): instinctCount updated after cap trim -----------------
header "Test 17 (v8.59.0): instinctCount=5 after cap trim from 6 to 5"
ROOT=$(make_repo)
jq '.instinctSummary = [
    {id:"inst-L017-old1", pattern:"old-1", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L017-old2", pattern:"old-2", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L017-old3", pattern:"old-3", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L017-old4", pattern:"old-4", confidence:0.5, type:"failure-lesson", errorCategory:"context"},
    {id:"inst-L017-old5", pattern:"old-5", confidence:0.5, type:"failure-lesson", errorCategory:"context"}
] | .instinctCount = 5' "$ROOT/.evolve/state.json" > "$ROOT/.evolve/state.json.tmp" && \
    mv "$ROOT/.evolve/state.json.tmp" "$ROOT/.evolve/state.json"
write_lesson "$ROOT" "inst-L017-new" "test-17-new-pattern" "reasoning"
write_handoff "$ROOT" 17 '["inst-L017-new"]'
EVOLVE_INSTINCT_SUMMARY_CAP=5 EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" "$ROOT/.evolve/runs/cycle-17" >/dev/null 2>&1
COUNT=$(jq -r '.instinctCount // 0' "$ROOT/.evolve/state.json")
LEN=$(jq -r '.instinctSummary | length' "$ROOT/.evolve/state.json")
if [ "$COUNT" = "5" ] && [ "$LEN" = "5" ]; then
    pass "instinctCount=5 after cap trim (Fix B: counter updated after cap)"
else
    fail "instinctCount=$COUNT instinctSummary.length=$LEN; expected both=5 after cap"
fi

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
