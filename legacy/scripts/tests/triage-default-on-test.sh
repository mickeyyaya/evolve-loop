#!/usr/bin/env bash
#
# triage-default-on-test.sh — v8.59.0 Layer T verification.
# Confirms the documentation + soft-WARN are in place for the Triage
# default-on flip. Doesn't run an actual cycle (that's the verification
# cycle's job); this is a static-config smoke test.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: orchestrator persona reflects default-on ---------------------
header "Test 1: orchestrator persona prompt is default-on"
ORCH="$REPO_ROOT/agents/evolve-orchestrator.md"
if grep -q "EVOLVE_TRIAGE_DISABLE" "$ORCH"; then
    pass "orchestrator persona references EVOLVE_TRIAGE_DISABLE (opt-out)"
else
    fail_ "orchestrator persona missing EVOLVE_TRIAGE_DISABLE reference"
fi
if grep -q "default-on" "$ORCH"; then
    pass "orchestrator persona mentions default-on"
else
    fail_ "orchestrator persona does not mention default-on"
fi

# --- Test 2: Triage persona reflects default-on ---------------------------
header "Test 2: agents/evolve-triage.md description is default-on"
TRIAGE="$REPO_ROOT/agents/evolve-triage.md"
if grep -q "default-on as of v8.59.0" "$TRIAGE"; then
    pass "Triage persona description is default-on"
else
    fail_ "Triage persona description not updated"
fi

# --- Test 3: phase-gate emits WARN when Triage skipped without opt-out ---
header "Test 3: gate_discover_to_build emits Layer T WARN"
PG="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
if grep -qE "Triage default-on.*v8.59|EVOLVE_TRIAGE_DISABLE.*triage-decision\.md" "$PG"; then
    pass "Layer T WARN block present in gate_discover_to_build"
else
    fail_ "Layer T WARN block missing"
fi

# --- Test 4: opt-out env var honored --------------------------------------
header "Test 4: EVOLVE_TRIAGE_DISABLE=1 suppresses the WARN"
# Inspect the phase-gate.sh code: the WARN should be conditional on
# EVOLVE_TRIAGE_DISABLE != 1
if grep -B1 -A8 "Triage default-on" "$PG" | grep -q 'EVOLVE_TRIAGE_DISABLE.*!= *"1"'; then
    pass "WARN gated on EVOLVE_TRIAGE_DISABLE != 1"
else
    fail_ "WARN not gated on disable flag — would always fire"
fi

# --- Test 5: CLAUDE.md documents the flip ---------------------------------
header "Test 5: CLAUDE.md describes Triage default-on"
CL="$REPO_ROOT/CLAUDE.md"
if grep -q "Triage default-on" "$CL"; then
    pass "CLAUDE.md references Triage default-on section"
else
    fail_ "CLAUDE.md missing Triage default-on documentation"
fi

# --- Test 6: memo persona reflects default-on -----------------------------
header "Test 6: agents/evolve-memo.md acknowledges Triage default-on"
MEMO="$REPO_ROOT/agents/evolve-memo.md"
if grep -q "EVOLVE_TRIAGE_DISABLE\|default-on" "$MEMO"; then
    pass "memo persona reflects Triage default-on"
else
    fail_ "memo persona still references EVOLVE_TRIAGE_ENABLED only"
fi

# --- Summary ----------------------------------------------------------------
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
