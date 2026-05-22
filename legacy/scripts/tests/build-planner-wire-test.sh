#!/usr/bin/env bash
# build-planner-wire-test.sh — Unit tests for Opt C cycle 1 (shadow) build-planner wiring.
#
# Verifies the 10 acceptance criteria from .evolve/runs/cycle-103/intent.md by
# delegating to the 9 ACS predicates in acs/cycle-103/*.sh PLUS direct schema
# assertions for files Builder must create.
#
# This is a TDD-Engineer RED-phase test: before Builder writes any production
# code, EVERY assertion in this file must FAIL. After Builder ships, EVERY
# assertion must PASS.
#
# Usage: bash legacy/scripts/tests/build-planner-wire-test.sh
# Exit 0 = all pass; non-zero = failures.
#
# Bash 3.2 compatible. No GNU-only flags.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$REPO_ROOT" || { echo "FATAL: cd to repo root failed" >&2; exit 1; }

PASS=0; FAIL=0; TOTAL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); TOTAL=$((TOTAL + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); TOTAL=$((TOTAL + 1)); }
header() { echo; echo "=== $* ==="; }

# ------------------------------------------------------------------
# Section 1 — Delegate to ACS predicates (the canonical AC contract).
# Each ACS predicate is run as a subprocess; exit 0 means GREEN, non-zero RED.
# ------------------------------------------------------------------

header "Section 1: ACS predicates (acs/regression-suite/cycle-103/*.sh)"

ACS_DIR="$REPO_ROOT/acs/regression-suite/cycle-103"
if [ ! -d "$ACS_DIR" ]; then
    fail "acs/cycle-103/ directory missing"
else
    for predicate in "$ACS_DIR"/*.sh; do
        [ -f "$predicate" ] || continue
        name=$(basename "$predicate")
        if bash "$predicate" >/dev/null 2>&1; then
            pass "$name (predicate GREEN)"
        else
            fail "$name (predicate RED — Builder has not made it pass yet)"
        fi
    done
fi

# ------------------------------------------------------------------
# Section 2 — Persona file structural assertions.
# ------------------------------------------------------------------

header "Section 2: agents/evolve-build-planner.md structure"

PERSONA="agents/evolve-build-planner.md"
if [ ! -f "$PERSONA" ]; then
    fail "$PERSONA exists on disk"
    fail "$PERSONA is git-tracked"
    fail "$PERSONA has 'name: evolve-build-planner' in frontmatter"
else
    pass "$PERSONA exists on disk"
    if git ls-files --error-unmatch "$PERSONA" >/dev/null 2>&1; then
        pass "$PERSONA is git-tracked (dual-check pass)"
    else
        fail "$PERSONA is git-tracked (dual-check fail — may be gitignored)"
    fi
    if grep -qE '^name:[[:space:]]+evolve-build-planner' "$PERSONA"; then
        pass "$PERSONA has 'name: evolve-build-planner' in frontmatter"
    else
        fail "$PERSONA has 'name: evolve-build-planner' in frontmatter"
    fi
fi

# ------------------------------------------------------------------
# Section 3 — Profile file structural assertions.
# ------------------------------------------------------------------

header "Section 3: .evolve/profiles/build-planner.json structure"

PROFILE=".evolve/profiles/build-planner.json"
if [ ! -f "$PROFILE" ]; then
    fail "$PROFILE exists on disk"
    fail "$PROFILE is valid JSON"
    fail "$PROFILE has parallel_eligible: false (single-writer invariant)"
else
    pass "$PROFILE exists on disk"
    if jq -e . "$PROFILE" >/dev/null 2>&1; then
        pass "$PROFILE is valid JSON"
        pe=$(jq -r '.parallel_eligible' "$PROFILE" 2>/dev/null)
        if [ "$pe" = "false" ]; then
            pass "$PROFILE has parallel_eligible: false (single-writer invariant)"
        else
            fail "$PROFILE parallel_eligible='$pe' (expected 'false')"
        fi
    else
        fail "$PROFILE is valid JSON"
        fail "$PROFILE has parallel_eligible: false"
    fi
fi

# ------------------------------------------------------------------
# Section 4 — Behavioral: list-phase-order.sh emits the new phase.
# Invokes the actual script under both REGISTRY flag values (not grep).
# ------------------------------------------------------------------

header "Section 4: legacy/scripts/dispatch/list-phase-order.sh behavioral"

LPO="legacy/scripts/dispatch/list-phase-order.sh"
if [ ! -f "$LPO" ]; then
    fail "$LPO exists"
else
    for flag in 1 0; do
        out=$(EVOLVE_USE_PHASE_REGISTRY="$flag" bash "$LPO" 2>/dev/null || true)
        if printf '%s\n' "$out" | grep -qx 'build-planner'; then
            pass "list-phase-order emits 'build-planner' under REGISTRY=$flag"
        else
            fail "list-phase-order omits 'build-planner' under REGISTRY=$flag"
        fi
    done
fi

# ------------------------------------------------------------------
# Section 5 — Behavioral: subagent-run.sh does NOT reject 'build-planner'.
# Invokes the script; the FAILURE REASON must not be "unknown agent".
# ------------------------------------------------------------------

header "Section 5: legacy/scripts/dispatch/subagent-run.sh behavioral"

SUBAGENT="legacy/scripts/dispatch/subagent-run.sh"
if [ ! -f "$SUBAGENT" ]; then
    fail "$SUBAGENT exists"
else
    # Correct positional invocation: <agent> <cycle> <workspace>
    err_out=$(bash "$SUBAGENT" build-planner 103 /tmp/nonexistent-cycle-103-probe 2>&1 || true)
    if printf '%s\n' "$err_out" | grep -qF "unknown agent: build-planner"; then
        fail "subagent-run.sh rejects 'build-planner' as unknown agent (allowlist gap)"
    else
        pass "subagent-run.sh accepts 'build-planner' (not rejected by allowlist)"
    fi
fi

# ------------------------------------------------------------------
# Section 6 — phase-gate.sh behavioral: dispatch case recognizes new gates.
# ------------------------------------------------------------------

header "Section 6: legacy/scripts/lifecycle/phase-gate.sh behavioral"

PHASE_GATE="legacy/scripts/lifecycle/phase-gate.sh"
if [ ! -f "$PHASE_GATE" ]; then
    fail "$PHASE_GATE exists"
else
    for gate_name in tdd-to-build-planner build-planner-to-build; do
        # Correct positional invocation: <gate> <cycle> <workspace>
        err_out=$(bash "$PHASE_GATE" "$gate_name" 103 .evolve/runs/cycle-103 2>&1 || true)
        if printf '%s\n' "$err_out" | grep -qiF "Unknown gate"; then
            fail "phase-gate.sh dispatch switch does not recognize '$gate_name'"
        else
            pass "phase-gate.sh dispatch switch recognizes '$gate_name'"
        fi
    done
fi

# ------------------------------------------------------------------
# Section 7 — ADR file existence and structure.
# ------------------------------------------------------------------

header "Section 7: docs/architecture/adr/0019-build-planner-phase.md"

ADR="docs/architecture/adr/0019-build-planner-phase.md"
if [ ! -f "$ADR" ]; then
    fail "$ADR exists on disk"
    fail "$ADR has '## Decision' section"
else
    pass "$ADR exists on disk"
    if grep -qFx "## Decision" "$ADR"; then
        pass "$ADR has '## Decision' section"
    else
        fail "$ADR has '## Decision' section"
    fi
fi

# ------------------------------------------------------------------
# Section 8 — Cycle-104 advisory-mode predicates (acs/cycle-104/*.sh)
# ------------------------------------------------------------------

header "Section 8: ACS predicates (acs/cycle-104/*.sh)"

ACS_104_DIR="$REPO_ROOT/acs/cycle-104"
if [ ! -d "$ACS_104_DIR" ]; then
    fail "acs/cycle-104/ directory missing"
else
    for predicate in "$ACS_104_DIR"/*.sh; do
        [ -f "$predicate" ] || continue
        name=$(basename "$predicate")
        if bash "$predicate" >/dev/null 2>&1; then
            pass "$name (predicate GREEN)"
        else
            fail "$name (predicate RED)"
        fi
    done
fi

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------

echo
echo "==========================================="
echo "Results: $PASS PASS, $FAIL FAIL, $TOTAL TOTAL"
echo "==========================================="

[ "$FAIL" -eq 0 ]
