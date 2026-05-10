#!/usr/bin/env bash
#
# persona-progressive-disclosure-test.sh — invariant tests for the
# persona Layer 1 / Layer 3 split.
#
# v8.64.0 Campaign D Cycle D1 — orchestrator persona only (proof of pattern).
# Cycles D2/D3 will extend to other personas.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

ORCH="$REPO_ROOT/agents/evolve-orchestrator.md"
ORCH_REF="$REPO_ROOT/agents/evolve-orchestrator-reference.md"

# --- Test 1: orchestrator reference file exists -----------------------------
header "Test 1: orchestrator reference file exists"
if [ -f "$ORCH_REF" ]; then
    pass "agents/evolve-orchestrator-reference.md present"
else
    fail_ "missing agents/evolve-orchestrator-reference.md"
fi

# --- Test 2: orchestrator persona has Reference Index section --------------
header "Test 2: orchestrator persona has Reference Index section"
if grep -q "^## Reference Index" "$ORCH"; then
    pass "## Reference Index section present"
else
    fail_ "missing ## Reference Index in orchestrator persona"
fi

# --- Test 3: Reference Index points at the reference file ------------------
header "Test 3: Reference Index links to reference file"
# At least one row should reference the reference file.
if grep -q "agents/evolve-orchestrator-reference.md" "$ORCH"; then
    pass "Reference Index links to evolve-orchestrator-reference.md"
else
    fail_ "Reference Index does not link to evolve-orchestrator-reference.md"
fi

# --- Test 4: reference file declares expected sections ---------------------
header "Test 4: reference file declares expected ## Section: <name> blocks"
declare -i ok=0
for section in operator-action-block-template failure-adapter-rationale operating-principles failure-modes-recovery; do
    if grep -q "^## Section: ${section}" "$ORCH_REF"; then
        ok=$((ok + 1))
    else
        echo "  MISSING section: $section"
    fi
done
if [ "$ok" = "4" ]; then
    pass "all 4 expected sections declared"
else
    fail_ "expected 4 sections, got $ok"
fi

# --- Test 5: orchestrator persona is under size cap ------------------------
# Pre-v8.64.0: 19030 bytes. Cap: 18000 bytes (modest headroom for future
# orchestrator content additions, but signals when more should be moved).
header "Test 5: orchestrator persona under 18000-byte cap"
size=$(wc -c < "$ORCH" | tr -d ' ')
if [ "$size" -lt 18000 ]; then
    pass "orchestrator persona = $size bytes (under 18000 cap)"
else
    fail_ "orchestrator persona = $size bytes EXCEEDS 18000 cap"
fi

# --- Test 6: reference file is small enough to load lazily ------------------
header "Test 6: reference file under 8KB hard cap"
ref_size=$(wc -c < "$ORCH_REF" | tr -d ' ')
if [ "$ref_size" -lt 8192 ]; then
    pass "reference file = $ref_size bytes (under 8192 cap)"
else
    fail_ "reference file = $ref_size bytes EXCEEDS 8192 cap"
fi

# --- Test 7: persona compact-Operating-Principles still numbered 1-5 -------
# Sanity check that the compact form preserves all five rules (so meaning
# is preserved across the split).
header "Test 7: compact Operating Principles list has 5 numbered items"
count=$(awk '/^## Operating Principles \(compact\)/{flag=1; next} /^## /{if (flag) flag=0} flag && /^[0-9]\. \*\*/' "$ORCH" | wc -l | tr -d ' ')
if [ "$count" = "5" ]; then
    pass "5 numbered principles present in compact form"
else
    fail_ "expected 5 numbered principles, got $count"
fi

# --- Test 8: full rationale is in Layer 3 (operating-principles section) ---
header "Test 8: Layer 3 has 5 numbered principles with full rationale"
count_l3=$(awk '/^## Section: operating-principles/{flag=1; next} /^## Section:/{if (flag) flag=0} flag && /^[0-9]\. \*\*/' "$ORCH_REF" | wc -l | tr -d ' ')
if [ "$count_l3" = "5" ]; then
    pass "5 numbered principles in Layer 3 reference"
else
    fail_ "expected 5 in Layer 3, got $count_l3"
fi

# --- Test 9 (D2): builder reference file exists -----------------------------
header "Test 9 (Cycle D2): builder reference file exists"
BUILDER_REF="$REPO_ROOT/agents/evolve-builder-reference.md"
if [ -f "$BUILDER_REF" ]; then
    pass "builder reference file present"
else
    fail_ "missing agents/evolve-builder-reference.md"
fi

# --- Test 10 (D2): builder persona has Reference Index ----------------------
header "Test 10 (Cycle D2): builder persona has Reference Index"
BUILDER="$REPO_ROOT/agents/evolve-builder.md"
if grep -q "^## Reference Index" "$BUILDER" && grep -q "evolve-builder-reference.md" "$BUILDER"; then
    pass "builder Reference Index linked to reference file"
else
    fail_ "builder missing Reference Index or link"
fi

# --- Test 11 (D2): builder reference declares expected sections ------------
header "Test 11 (Cycle D2): builder reference declares e2e + capability + self-review sections"
declare -i bok=0
for s in e2e-test-generation capability-gap-detection optional-self-review; do
    if grep -q "^## Section: ${s}" "$BUILDER_REF"; then
        bok=$((bok + 1))
    else
        echo "  MISSING: $s"
    fi
done
if [ "$bok" = "3" ]; then
    pass "all 3 builder reference sections declared"
else
    fail_ "expected 3, got $bok"
fi

# --- Test 12 (D2): auditor reference file exists ----------------------------
header "Test 12 (Cycle D2): auditor reference file exists"
AUDITOR_REF="$REPO_ROOT/agents/evolve-auditor-reference.md"
if [ -f "$AUDITOR_REF" ]; then
    pass "auditor reference file present"
else
    fail_ "missing agents/evolve-auditor-reference.md"
fi

# --- Test 13 (D2): auditor persona has Reference Index ---------------------
header "Test 13 (Cycle D2): auditor persona has Reference Index"
AUDITOR="$REPO_ROOT/agents/evolve-auditor.md"
if grep -q "^## Reference Index" "$AUDITOR" && grep -q "evolve-auditor-reference.md" "$AUDITOR"; then
    pass "auditor Reference Index linked to reference file"
else
    fail_ "auditor missing Reference Index or link"
fi

# --- Test 14 (D2): auditor reference declares adaptive-strictness ----------
header "Test 14 (Cycle D2): auditor reference declares adaptive-strictness section"
if grep -q "^## Section: adaptive-strictness" "$AUDITOR_REF"; then
    pass "auditor reference has adaptive-strictness section"
else
    fail_ "auditor reference missing adaptive-strictness section"
fi

# --- Test 15 (D2): no persona exceeds size cap -----------------------------
header "Test 15 (Cycle D2): no persona exceeds size cap"
declare -i cap=18000
declare -i over=0
for f in evolve-orchestrator evolve-builder evolve-auditor; do
    sz=$(wc -c < "$REPO_ROOT/agents/${f}.md" | tr -d ' ')
    if [ "$sz" -ge "$cap" ]; then
        echo "  OVER: ${f}.md = $sz bytes"
        over=$((over + 1))
    fi
done
if [ "$over" = "0" ]; then
    pass "all 3 split personas under $cap bytes"
else
    fail_ "$over persona(s) over cap"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ]
