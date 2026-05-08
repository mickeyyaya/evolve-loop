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

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
