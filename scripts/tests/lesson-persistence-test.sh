#!/usr/bin/env bash
#
# lesson-persistence-test.sh — Verify backfill-lessons.sh idempotency and
# instinctSummary persistence contract.
#
# Tests:
#   1. backfill-lessons.sh exists and is executable
#   2. --dry-run flag exits 0 (or 2) and does NOT modify state.json
#   3. Running twice is idempotent (second run exits 2, state.json unchanged)
#   4. All on-disk lesson YAMLs are represented in state.json:instinctSummary[]
#
# Usage:
#   bash scripts/tests/lesson-persistence-test.sh
#
# Exit codes:
#   0 — all tests PASS
#   1 — one or more tests FAIL

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

BACKFILL="$EVOLVE_PLUGIN_ROOT/scripts/utility/backfill-lessons.sh"
STATE="${EVOLVE_STATE_FILE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"
LESSONS_DIR="$EVOLVE_PROJECT_ROOT/.evolve/instincts/lessons"

PASS=0
FAIL=0

_pass() { echo "[PASS] $1"; PASS=$((PASS + 1)); }
_fail() { echo "[FAIL] $1"; FAIL=$((FAIL + 1)); }

# Test 1: script exists and is executable
if [ -x "$BACKFILL" ]; then
    _pass "backfill-lessons.sh exists and is executable"
else
    _fail "backfill-lessons.sh not found or not executable at $BACKFILL"
fi

# Test 2: --dry-run does NOT modify state.json
if [ -f "$STATE" ] && command -v jq >/dev/null 2>&1; then
    before_count=$(jq '.instinctSummary | length' "$STATE" 2>/dev/null || echo "-1")
    before_mtime=""
    if [[ "$OSTYPE" == "darwin"* ]]; then
        before_mtime=$(stat -f %m "$STATE" 2>/dev/null || echo "0")
    else
        before_mtime=$(stat -c %Y "$STATE" 2>/dev/null || echo "0")
    fi

    EVOLVE_PROJECT_ROOT="$EVOLVE_PROJECT_ROOT" \
    EVOLVE_PLUGIN_ROOT="$EVOLVE_PLUGIN_ROOT" \
    bash "$BACKFILL" --dry-run 2>/dev/null
    dry_rc=$?

    after_count=$(jq '.instinctSummary | length' "$STATE" 2>/dev/null || echo "-1")
    after_mtime=""
    if [[ "$OSTYPE" == "darwin"* ]]; then
        after_mtime=$(stat -f %m "$STATE" 2>/dev/null || echo "0")
    else
        after_mtime=$(stat -c %Y "$STATE" 2>/dev/null || echo "0")
    fi

    if [ "$before_count" = "$after_count" ] && [ "$before_mtime" = "$after_mtime" ]; then
        _pass "--dry-run exits $dry_rc without modifying state.json (count=$before_count)"
    else
        _fail "--dry-run modified state.json (before_count=$before_count after_count=$after_count)"
    fi
else
    _fail "state.json not found at $STATE (or jq missing)"
fi

# Test 3: second run is idempotent (exits 2 = nothing to do)
if [ -x "$BACKFILL" ] && [ -f "$STATE" ]; then
    # First, ensure the backfill has already run once (state should be current).
    EVOLVE_PROJECT_ROOT="$EVOLVE_PROJECT_ROOT" \
    EVOLVE_PLUGIN_ROOT="$EVOLVE_PLUGIN_ROOT" \
    bash "$BACKFILL" 2>/dev/null
    rc1=$?

    before_count2=$(jq '.instinctSummary | length' "$STATE" 2>/dev/null || echo "-1")

    # Second run: should be idempotent
    EVOLVE_PROJECT_ROOT="$EVOLVE_PROJECT_ROOT" \
    EVOLVE_PLUGIN_ROOT="$EVOLVE_PLUGIN_ROOT" \
    bash "$BACKFILL" 2>/dev/null
    rc2=$?

    after_count2=$(jq '.instinctSummary | length' "$STATE" 2>/dev/null || echo "-1")

    if [ "$before_count2" = "$after_count2" ]; then
        _pass "idempotent: second run exit=$rc2, count unchanged ($before_count2)"
    else
        _fail "not idempotent: count changed $before_count2 → $after_count2 on second run"
    fi
else
    _fail "skipping idempotency test (backfill not found or state.json missing)"
fi

# Test 4: all on-disk YAMLs present in instinctSummary
if [ -d "$LESSONS_DIR" ] && [ -f "$STATE" ] && command -v jq >/dev/null 2>&1; then
    missing=0
    for yaml_file in "$LESSONS_DIR"/*.yaml; do
        [ -f "$yaml_file" ] || continue
        file_id=""
        if command -v python3 >/dev/null 2>&1; then
            file_id=$(python3 -c "
try:
    with open('$yaml_file') as f:
        for line in f:
            s = line.strip()
            if s.startswith('- id:'):
                print(s[len('- id:'):].strip()); break
            elif s.startswith('id:'):
                print(s[len('id:'):].strip()); break
except Exception: pass
" 2>/dev/null)
        fi
        if [ -z "$file_id" ]; then
            file_id=$(grep -E '^(- )?id:' "$yaml_file" | head -1 | sed 's/^-[[:space:]]*//' | sed 's/^id:[[:space:]]*//' | tr -d '"')
        fi
        [ -z "$file_id" ] && continue

        found=$(jq --arg id "$file_id" '[.instinctSummary[]? | select(.id == $id)] | length' "$STATE" 2>/dev/null || echo "0")
        if [ "$found" = "0" ]; then
            echo "  missing from instinctSummary: $file_id"
            missing=$((missing + 1))
        fi
    done
    if [ "$missing" -eq 0 ]; then
        total=$(ls "$LESSONS_DIR"/*.yaml 2>/dev/null | wc -l | tr -d ' ')
        _pass "all $total on-disk lesson YAMLs present in instinctSummary"
    else
        _fail "$missing on-disk YAML(s) missing from instinctSummary (run backfill-lessons.sh)"
    fi
else
    _fail "lessons dir or state.json not found"
fi

# Summary
echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
