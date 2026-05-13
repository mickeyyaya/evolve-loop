#!/usr/bin/env bash
#
# reconcile-carryover-todos-test.sh — v8.57.0 Layer D smoke tests.
# Verifies scripts/lifecycle/reconcile-carryover-todos.sh correctly
# updates cycles_unpicked per cycle and archives at threshold.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HELPER="$REPO_ROOT/scripts/lifecycle/reconcile-carryover-todos.sh"
SCRATCH=$(mktemp -d)

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Build a fake project root with state.json containing carryoverTodos.
make_repo_with_todos() {
    local todos_json="$1"
    local root="$SCRATCH/repo-$RANDOM"
    mkdir -p "$root/.evolve/runs/cycle-1"
    cat > "$root/.evolve/state.json" <<EOF
{
  "instinctSummary": [],
  "carryoverTodos": $todos_json,
  "failedApproaches": []
}
EOF
    echo "$root"
}

# Substantive scout-report with a Carryover Decisions section.
write_scout_report_with_decisions() {
    local ws="$1"
    shift
    cat > "$ws/scout-report.md" <<'HDR'
<!-- challenge: test -->
# Cycle Scout Report
## Discovery Summary
- Files analyzed: 12
## Selected Tasks
### Task 1: stub
- Slug: stub
HDR
    echo "## Carryover Decisions" >> "$ws/scout-report.md"
    for line in "$@"; do
        echo "- $line" >> "$ws/scout-report.md"
    done
}

# --- Test 1: helper exists and is executable ------------------------------
header "Test 1: reconcile-carryover-todos.sh exists and is executable"
[ -f "$HELPER" ] && pass "helper file exists" || fail "missing $HELPER"
[ -x "$HELPER" ] && pass "helper is executable" || fail "$HELPER not executable"

# --- Test 2: 'include' decision resets cycles_unpicked + drops on PASS ----
header "Test 2: include + PASS verdict drops todo from list"
TODOS='[{"id":"todo-1","action":"x","priority":"high","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":2}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report_with_decisions "$WS" "todo-1: include, reason: aligns this cycle"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json" 2>/dev/null)
[ "$LEN" = "0" ] && pass "todo-1 dropped on PASS+include" || fail "expected 0 todos, got $LEN"

# --- Test 3: 'defer' increments cycles_unpicked --------------------------
header "Test 3: defer increments cycles_unpicked"
TODOS='[{"id":"todo-2","action":"x","priority":"medium","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report_with_decisions "$WS" "todo-2: defer, reason: out of scope"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
CU=$(jq -r '.carryoverTodos[] | select(.id=="todo-2") | .cycles_unpicked' "$ROOT/.evolve/state.json")
[ "$CU" = "1" ] && pass "cycles_unpicked incremented to 1" || fail "expected 1, got $CU"

# --- Test 4: 'drop' archives immediately ---------------------------------
header "Test 4: drop archives immediately"
TODOS='[{"id":"todo-3","action":"x","priority":"low","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report_with_decisions "$WS" "todo-3: drop, reason: duplicate of todo-1"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "0" ] && pass "todo-3 removed from active carryoverTodos" || fail "expected 0, got $LEN"
ARCHIVE="$ROOT/.evolve/archive/lessons/carryover-todos-archive.jsonl"
if [ -f "$ARCHIVE" ] && grep -q '"id":"todo-3"' "$ARCHIVE"; then
    pass "todo-3 written to archive jsonl"
else
    fail "archive missing or todo-3 not present: $ARCHIVE"
fi

# --- Test 5: cycles_unpicked >= EVOLVE_CARRYOVER_TODO_MAX_UNPICKED archives -
header "Test 5: cycles_unpicked >= max threshold (default 3) auto-archives"
TODOS='[{"id":"todo-4","action":"x","priority":"medium","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":2}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report_with_decisions "$WS" "todo-4: defer, reason: still out of scope"
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 4 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "0" ] && pass "todo-4 auto-archived (cycles_unpicked hit 3)" || fail "expected 0, got $LEN"
ARCHIVE="$ROOT/.evolve/archive/lessons/carryover-todos-archive.jsonl"
grep -q '"id":"todo-4"' "$ARCHIVE" 2>/dev/null && pass "todo-4 in archive" || fail "todo-4 not archived"

# --- Test 6: env override of threshold ------------------------------------
header "Test 6: EVOLVE_CARRYOVER_TODO_MAX_UNPICKED overrides default"
TODOS='[{"id":"todo-5","action":"x","priority":"high","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report_with_decisions "$WS" "todo-5: defer, reason: not yet"
# threshold=1: first defer = archive
EVOLVE_CARRYOVER_TODO_MAX_UNPICKED=1 EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "0" ] && pass "env override triggers archive at threshold=1" || fail "expected 0, got $LEN"

# --- Test 7: missing scout-report decisions section (defensive WARN+inc) --
header "Test 7: todo not mentioned anywhere increments cycles_unpicked + WARN"
TODOS='[{"id":"todo-6","action":"x","priority":"low","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
# Scout report has Carryover Decisions section but doesn't mention todo-6
write_scout_report_with_decisions "$WS" "todo-other: include, reason: dummy"
WARN_OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS 2>&1 >/dev/null)
CU=$(jq -r '.carryoverTodos[] | select(.id=="todo-6") | .cycles_unpicked' "$ROOT/.evolve/state.json")
[ "$CU" = "1" ] && pass "todo-6 cycles_unpicked incremented defensively" || fail "expected 1, got $CU"
echo "$WARN_OUT" | grep -qi "WARN.*todo-6\|todo-6.*not.*seen" && pass "WARN emitted for unseen todo" || \
    fail "no WARN about unseen todo-6; got: $WARN_OUT"

# --- Test 8: triage-decision.md top_n is treated as 'include' ------------
header "Test 8: triage top_n is treated as 'include' (no scout decision needed)"
TODOS='[{"id":"todo-7","action":"x","priority":"high","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
# No scout-report at all — Triage was the source
cat > "$WS/triage-decision.md" <<'EOF'
<!-- challenge-token: t -->
# Triage Decision
cycle_size_estimate: small
## top_n
- todo-7: do the thing
## deferred
## dropped
EOF
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "0" ] && pass "todo-7 dropped (triage top_n + PASS)" || fail "expected 0, got $LEN"

# --- Test 9: table-format triage top_n is treated as 'include' (c38-D fix) ---
header "Test 9: table-format triage top_n drops todo on PASS (c38-D-triage-parse fix)"
TODOS='[{"id":"test-todo-1","action":"x","priority":"high","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":2}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
cat > "$WS/triage-decision.md" <<'EOF'
<!-- challenge-token: t -->
# Triage Decision — Cycle 1

## Top N — Included

| Rank | ID | Priority | Weight | LoC Est | Rationale |
|------|----|----------|--------|---------|-----------|
| 1 | `test-todo-1` | HIGH | 0.92 | ~100 | Test fixture for table-format parsing |

## Deferred

| ID | Priority | Defer Reason | Target Cycle |
|----|----------|--------------|--------------|

## Dropped

None.
EOF
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
LEN=$(jq -r '.carryoverTodos | length' "$ROOT/.evolve/state.json")
[ "$LEN" = "0" ] && pass "test-todo-1 dropped (table-format triage top_n + PASS)" \
    || fail "expected 0 todos, got $LEN (table-format triage parse may be broken)"

# --- Test 10: table-format triage deferred increments cycles_unpicked -------
header "Test 10: table-format triage deferred increments cycles_unpicked"
TODOS='[{"id":"test-todo-2","action":"x","priority":"medium","evidence_pointer":"y","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}]'
ROOT=$(make_repo_with_todos "$TODOS")
WS="$ROOT/.evolve/runs/cycle-1"
cat > "$WS/triage-decision.md" <<'EOF'
<!-- challenge-token: t -->
# Triage Decision — Cycle 1

## Top N — Included

| Rank | ID | Priority |
|------|----|----------|

## Deferred

| ID | Priority | Defer Reason | Target Cycle |
|----|----------|--------------|--------------|
| `test-todo-2` | MEDIUM | out of scope this cycle | 3 |

## Dropped

None.
EOF
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" --cycle 2 --workspace "$WS" --verdict PASS >/dev/null 2>&1 || true
CU=$(jq -r '.carryoverTodos[] | select(.id=="test-todo-2") | .cycles_unpicked' "$ROOT/.evolve/state.json")
[ "$CU" = "1" ] && pass "test-todo-2 cycles_unpicked incremented to 1 (table-format defer)" \
    || fail "expected 1, got $CU"

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "$PASS pass / $FAIL fail"
echo "==========================================="
[ "$FAIL" -eq 0 ]
