#!/usr/bin/env bash
#
# aggregator-test.sh — Unit tests for scripts/aggregator.sh.
#
# aggregator.sh merges N worker artifacts produced by fanout-dispatch.sh
# into a single canonical phase artifact. It is a PURE SHELL merge — no LLM
# call — so it cannot be coerced into accepting forged worker output. The
# trust kernel still binds the AGGREGATE output via phase-gate.sh's existing
# check_subagent_ledger_match.
#
# Per-phase merge rules:
#   scout / research / discover → concat with "## Worker: <name>" headers
#   audit                       → ALL-PASS verdict (any FAIL fails the aggregate)
#   learn / retrospective       → union of "## Lesson:" sections, deduplicated
#
# Tests cover:
#   1. script exists + executable
#   2. usage error: no workers → exit 2
#   3. scout phase: 3 worker files → output has 3 sections, exit 0
#   4. scout phase: missing worker file → fail with clear error
#   5. scout phase: empty worker file → fail
#   6. audit phase: 3 PASS verdicts → aggregate PASS, exit 0
#   7. audit phase: 2 PASS + 1 FAIL → aggregate FAIL, exit 1
#   8. learn phase: dedup repeated lessons across workers
#
# Bash 3.2 compatible per CLAUDE.md.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/aggregator.sh"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

fresh_workspace() {
    mktemp -d -t aggregator-test.XXXXXX
}

# --- Test 1: script exists ---------------------------------------------------
header "Test 1: scripts/aggregator.sh exists and is executable"
if [ -f "$SCRIPT" ] && [ -x "$SCRIPT" ]; then
    pass "$SCRIPT present and executable"
else
    fail_ "$SCRIPT missing or not executable"
    echo
    echo "=== Summary ==="
    echo "  PASS: $PASS"
    echo "  FAIL: $FAIL"
    exit 1
fi

# --- Test 2: usage error on no workers ---------------------------------------
header "Test 2: no workers → exit 2 (usage error)"
WS=$(fresh_workspace)
rc=0
"$SCRIPT" scout "$WS/out.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "2" ]; then
    pass "no-workers → exit 2"
else
    fail_ "expected exit 2, got $rc"
fi
rm -rf "$WS"

# --- Test 3: scout phase merges 3 workers ------------------------------------
header "Test 3: scout phase, 3 worker files → output has 3 sections"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
echo "Found 12 files matching pattern X." > "$WS/workers/scout-codebase.md"
echo "Web search returned 4 prior implementations." > "$WS/workers/scout-research.md"
echo "Designed 2 evals: eval-A, eval-B." > "$WS/workers/scout-evals.md"
rc=0
"$SCRIPT" scout "$WS/scout-report.md" \
    "$WS/workers/scout-codebase.md" \
    "$WS/workers/scout-research.md" \
    "$WS/workers/scout-evals.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then
    pass "scout merge → exit 0"
else
    fail_ "expected exit 0, got $rc"
fi
SECTIONS=$(grep -c "^## Worker:" "$WS/scout-report.md" 2>/dev/null || echo 0)
if [ "$SECTIONS" = "3" ]; then
    pass "output has 3 worker sections"
else
    fail_ "expected 3 ## Worker: headers, got $SECTIONS"
fi
if grep -q "Found 12 files" "$WS/scout-report.md" \
   && grep -q "Web search returned" "$WS/scout-report.md" \
   && grep -q "Designed 2 evals" "$WS/scout-report.md"; then
    pass "all worker bodies present in output"
else
    fail_ "worker bodies missing from output"
fi
rm -rf "$WS"

# --- Test 4: missing worker file ---------------------------------------------
header "Test 4: scout phase, missing worker file → fail with clear error"
WS=$(fresh_workspace)
rc=0
"$SCRIPT" scout "$WS/out.md" "$WS/does-not-exist.md" >"$WS/stdout" 2>"$WS/stderr" || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "missing file → non-zero exit"
else
    fail_ "expected non-zero exit on missing file, got 0"
fi
if grep -qi "not found\|missing\|does not exist" "$WS/stderr" 2>/dev/null; then
    pass "stderr mentions missing file"
else
    fail_ "stderr should explain missing file; got: $(cat "$WS/stderr")"
fi
rm -rf "$WS"

# --- Test 5: empty worker file -----------------------------------------------
header "Test 5: scout phase, empty worker file → fail"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
: > "$WS/workers/empty.md"
rc=0
"$SCRIPT" scout "$WS/out.md" "$WS/workers/empty.md" >/dev/null 2>"$WS/stderr" || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "empty worker → non-zero exit"
else
    fail_ "expected non-zero exit on empty file"
fi
rm -rf "$WS"

# --- Test 6: audit phase, all PASS -------------------------------------------
header "Test 6: audit phase, 3 PASS verdicts → aggregate PASS"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
{ echo "Verdict: PASS"; echo "5 evals replayed."; } > "$WS/workers/audit-eval.md"
{ echo "Verdict: PASS"; echo "lint clean."; }        > "$WS/workers/audit-lint.md"
{ echo "Verdict: PASS"; echo "12/12 regression."; }  > "$WS/workers/audit-regression.md"
rc=0
"$SCRIPT" audit "$WS/audit-report.md" \
    "$WS/workers/audit-eval.md" \
    "$WS/workers/audit-lint.md" \
    "$WS/workers/audit-regression.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then
    pass "all PASS → exit 0"
else
    fail_ "expected exit 0, got $rc"
fi
if grep -q "^Verdict: PASS" "$WS/audit-report.md"; then
    pass "aggregate Verdict: PASS"
else
    fail_ "expected 'Verdict: PASS' in output; got:"
    head -5 "$WS/audit-report.md"
fi
rm -rf "$WS"

# --- Test 7: audit phase, mixed PASS/FAIL ------------------------------------
header "Test 7: audit phase, 2 PASS + 1 FAIL → aggregate FAIL"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
{ echo "Verdict: PASS"; echo "5 evals replayed."; } > "$WS/workers/audit-eval.md"
{ echo "Verdict: FAIL"; echo "lint errors found."; } > "$WS/workers/audit-lint.md"
{ echo "Verdict: PASS"; echo "12/12 regression."; }  > "$WS/workers/audit-regression.md"
rc=0
"$SCRIPT" audit "$WS/audit-report.md" \
    "$WS/workers/audit-eval.md" \
    "$WS/workers/audit-lint.md" \
    "$WS/workers/audit-regression.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "any FAIL → non-zero exit"
else
    fail_ "expected non-zero exit on FAIL, got 0"
fi
if grep -q "^Verdict: FAIL" "$WS/audit-report.md"; then
    pass "aggregate Verdict: FAIL"
else
    fail_ "expected 'Verdict: FAIL' in output"
fi
rm -rf "$WS"

# --- Test 8a: plan-review phase, all PROCEED → aggregate PROCEED -------------
header "Test 8a: plan-review, 4 PROCEED lenses (avg >= 7) → Verdict: PROCEED"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
{ echo "Score: 9"; echo "Verdict: PROCEED"; echo "ambitious enough"; }   > "$WS/workers/plan-ceo.md"
{ echo "Score: 8"; echo "Verdict: PROCEED"; echo "tests feasible"; }     > "$WS/workers/plan-eng.md"
{ echo "Score: 8"; echo "Verdict: PROCEED"; echo "elegant API"; }        > "$WS/workers/plan-design.md"
{ echo "Score: 9"; echo "Verdict: PROCEED"; echo "preserves kernel"; }   > "$WS/workers/plan-security.md"
rc=0
"$SCRIPT" plan-review "$WS/plan-review.md" \
    "$WS/workers/plan-ceo.md" "$WS/workers/plan-eng.md" \
    "$WS/workers/plan-design.md" "$WS/workers/plan-security.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then
    pass "all PROCEED lenses → exit 0"
else
    fail_ "expected exit 0, got $rc"
fi
if grep -q "^Verdict: PROCEED" "$WS/plan-review.md"; then
    pass "aggregate Verdict: PROCEED"
else
    fail_ "expected PROCEED; got: $(head -3 "$WS/plan-review.md")"
fi
rm -rf "$WS"

# --- Test 8b: plan-review with one weak lens → Verdict: REVISE ---------------
header "Test 8b: plan-review, avg >= 5 + one lens < 5 → Verdict: REVISE"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
{ echo "Score: 8"; echo "Verdict: PROCEED"; }           > "$WS/workers/plan-ceo.md"
{ echo "Score: 4"; echo "Verdict: REVISE"; echo "test gaps"; } > "$WS/workers/plan-eng.md"
{ echo "Score: 7"; echo "Verdict: PROCEED"; }           > "$WS/workers/plan-design.md"
{ echo "Score: 6"; echo "Verdict: PROCEED"; }           > "$WS/workers/plan-security.md"
rc=0
"$SCRIPT" plan-review "$WS/plan-review.md" \
    "$WS/workers/plan-ceo.md" "$WS/workers/plan-eng.md" \
    "$WS/workers/plan-design.md" "$WS/workers/plan-security.md" >/dev/null 2>&1 || rc=$?
# REVISE is not a "fail" — exit code stays 0; the verdict is in the artifact.
if grep -q "^Verdict: REVISE" "$WS/plan-review.md"; then
    pass "aggregate Verdict: REVISE (weak lens triggers revision)"
else
    fail_ "expected REVISE; got: $(head -3 "$WS/plan-review.md")"
fi
rm -rf "$WS"

# --- Test 8c: plan-review with low average → Verdict: ABORT ------------------
header "Test 8c: plan-review, avg < 5 → Verdict: ABORT"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
{ echo "Score: 3"; echo "Verdict: ABORT"; echo "scope wrong"; } > "$WS/workers/plan-ceo.md"
{ echo "Score: 4"; echo "Verdict: REVISE"; }                      > "$WS/workers/plan-eng.md"
{ echo "Score: 3"; echo "Verdict: ABORT"; }                      > "$WS/workers/plan-design.md"
{ echo "Score: 5"; echo "Verdict: PROCEED"; }                    > "$WS/workers/plan-security.md"
rc=0
"$SCRIPT" plan-review "$WS/plan-review.md" \
    "$WS/workers/plan-ceo.md" "$WS/workers/plan-eng.md" \
    "$WS/workers/plan-design.md" "$WS/workers/plan-security.md" >/dev/null 2>&1 || rc=$?
# ABORT is a fail-state; exit 1.
if [ "$rc" -ne 0 ]; then
    pass "ABORT → non-zero exit"
else
    fail_ "expected non-zero exit on ABORT"
fi
if grep -q "^Verdict: ABORT" "$WS/plan-review.md"; then
    pass "aggregate Verdict: ABORT"
else
    fail_ "expected ABORT; got: $(head -3 "$WS/plan-review.md")"
fi
rm -rf "$WS"

# --- Test 8d: any explicit ABORT verdict propagates --------------------------
header "Test 8d: plan-review, any explicit ABORT lens → Verdict: ABORT"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
# Average is 7 (would normally PROCEED), but security says ABORT.
{ echo "Score: 9"; echo "Verdict: PROCEED"; }    > "$WS/workers/plan-ceo.md"
{ echo "Score: 8"; echo "Verdict: PROCEED"; }    > "$WS/workers/plan-eng.md"
{ echo "Score: 7"; echo "Verdict: PROCEED"; }    > "$WS/workers/plan-design.md"
{ echo "Score: 4"; echo "Verdict: ABORT"; echo "weakens sandbox"; } > "$WS/workers/plan-security.md"
rc=0
"$SCRIPT" plan-review "$WS/plan-review.md" \
    "$WS/workers/plan-ceo.md" "$WS/workers/plan-eng.md" \
    "$WS/workers/plan-design.md" "$WS/workers/plan-security.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "explicit ABORT → non-zero exit"
else
    fail_ "expected non-zero exit"
fi
if grep -q "^Verdict: ABORT" "$WS/plan-review.md"; then
    pass "explicit ABORT propagates regardless of average"
else
    fail_ "expected ABORT; got: $(head -3 "$WS/plan-review.md")"
fi
rm -rf "$WS"

# --- Test 8: learn phase, dedup repeated lessons -----------------------------
header "Test 8: learn phase, dedup repeated lessons across workers"
WS=$(fresh_workspace)
mkdir -p "$WS/workers"
{
    echo "## Lesson: tests are slow"
    echo "## Lesson: cache is helpful"
} > "$WS/workers/retro-instinct.md"
{
    echo "## Lesson: cache is helpful"
    echo "## Lesson: deps drift"
} > "$WS/workers/retro-gene.md"
rc=0
"$SCRIPT" learn "$WS/retrospective-report.md" \
    "$WS/workers/retro-instinct.md" \
    "$WS/workers/retro-gene.md" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then
    pass "learn merge → exit 0"
else
    fail_ "expected exit 0, got $rc"
fi
LESSONS=$(grep -c "^## Lesson:" "$WS/retrospective-report.md" 2>/dev/null || echo 0)
if [ "$LESSONS" = "3" ]; then
    pass "3 unique lessons (dedup worked: tests/cache/deps)"
else
    fail_ "expected 3 unique lessons after dedup, got $LESSONS"
    cat "$WS/retrospective-report.md"
fi
rm -rf "$WS"

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
[ "$FAIL" -eq 0 ]
