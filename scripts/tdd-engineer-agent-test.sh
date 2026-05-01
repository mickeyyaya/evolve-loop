#!/usr/bin/env bash
#
# tdd-engineer-agent-test.sh — TDD-first verification that the TDD-Engineer
# agent is fully wired into the swarm pipeline.
#
# Tests (all 6 must FAIL before Phase 1 implementation, all 6 must PASS after):
#   1. agents/evolve-tdd-engineer.md exists in main checkout
#   2. Frontmatter contains required `perspective:` and `output-format:` per
#      agents/agent-templates.md schema (cycle-16 persona-field convention)
#   3. .evolve/profiles/tdd-engineer.json exists and validates as JSON
#   4. scripts/subagent-run.sh agent regex (line ~202) accepts `tdd-engineer`
#   5. scripts/guards/phase-gate-precondition.sh has a `tdd)` case allowing
#      `tdd-engineer` (new phase between `discover` and `build`)
#   6. agents/agent-templates.md mentions "TDD Engineer" or "tdd-engineer"
#      in its agent-roster documentation
#
# This is a pure static test — no subagent invocations, no LLM cost.
#
# Usage: bash scripts/tdd-engineer-agent-test.sh
# Exit:  0 if all assertions pass; non-zero if any fail.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
AGENT_FILE="$REPO_ROOT/agents/evolve-tdd-engineer.md"
PROFILE_FILE="$REPO_ROOT/.evolve/profiles/tdd-engineer.json"
SUBAGENT_RUN="$REPO_ROOT/scripts/subagent-run.sh"
PHASE_GATE="$REPO_ROOT/scripts/guards/phase-gate-precondition.sh"
TEMPLATES="$REPO_ROOT/agents/agent-templates.md"

PASS=0
FAIL=0

pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: agent file exists -----------------------------------------------
header "Test 1: agents/evolve-tdd-engineer.md exists"
if [ -f "$AGENT_FILE" ]; then
    pass "$AGENT_FILE present"
else
    fail "$AGENT_FILE missing"
fi

# --- Test 2: frontmatter has required persona fields -------------------------
header "Test 2: Frontmatter contains perspective + output-format"
if [ ! -f "$AGENT_FILE" ]; then
    fail "cannot check frontmatter — $AGENT_FILE missing"
else
    if grep -qE '^perspective:[[:space:]]*' "$AGENT_FILE"; then
        pass "perspective: field present"
    else
        fail "perspective: field missing from frontmatter"
    fi
    if grep -qE '^output-format:[[:space:]]*' "$AGENT_FILE"; then
        pass "output-format: field present"
    else
        fail "output-format: field missing from frontmatter"
    fi
fi

# --- Test 3: profile file exists + valid JSON --------------------------------
header "Test 3: .evolve/profiles/tdd-engineer.json exists and validates"
if [ ! -f "$PROFILE_FILE" ]; then
    fail "$PROFILE_FILE missing"
else
    if jq empty "$PROFILE_FILE" 2>/dev/null; then
        pass "$PROFILE_FILE is valid JSON"
    else
        fail "$PROFILE_FILE is not valid JSON"
    fi
    # Profile must have role: tdd-engineer
    role=$(jq -r '.role // empty' "$PROFILE_FILE" 2>/dev/null)
    if [ "$role" = "tdd-engineer" ]; then
        pass "profile.role = tdd-engineer"
    else
        fail "profile.role = '$role' (expected tdd-engineer)"
    fi
fi

# --- Test 4: subagent-run.sh accepts tdd-engineer in agent regex -------------
header "Test 4: subagent-run.sh agent regex accepts tdd-engineer"
if grep -qE '\^\(.*tdd-engineer.*\)\$' "$SUBAGENT_RUN"; then
    pass "tdd-engineer present in subagent-run.sh agent regex"
else
    fail "tdd-engineer NOT in subagent-run.sh agent regex (expected match against pattern '^(...tdd-engineer...)$')"
fi

# --- Test 5: phase-gate-precondition.sh has `tdd` phase + tdd-engineer -----
header "Test 5: phase-gate-precondition.sh recognizes tdd phase + agent"
if grep -qE '^[[:space:]]*tdd\)' "$PHASE_GATE"; then
    pass "phase-gate has 'tdd)' case"
else
    fail "phase-gate-precondition.sh missing 'tdd)' case in PHASE switch"
fi
# tdd-engineer must be in EXPECTED for at least one phase (the 'tdd' phase)
if grep -A 1 -E '^[[:space:]]*tdd\)' "$PHASE_GATE" 2>/dev/null | grep -q 'tdd-engineer'; then
    pass "tdd-engineer allowed in tdd phase"
else
    fail "tdd-engineer NOT allowed in tdd phase EXPECTED list"
fi

# --- Test 6: agent-templates.md documents TDD-Engineer ----------------------
header "Test 6: agents/agent-templates.md mentions TDD-Engineer"
if grep -qiE 'tdd[ -]engineer' "$TEMPLATES"; then
    pass "agent-templates.md references TDD-Engineer / tdd-engineer"
else
    fail "agent-templates.md does NOT reference TDD-Engineer"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "  ALL CHECKS PASS"
    exit 0
else
    echo "  $FAIL CHECK(S) FAILED"
    exit 1
fi
