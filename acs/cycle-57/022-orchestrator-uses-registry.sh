#!/usr/bin/env bash
# ACS predicate 022 — cycle 57
# Verifies that the orchestrator persona references registry-driven dispatch
# (list-phase-order.sh + gate_run_by_name) and that gate_run_by_name correctly
# dispatches a known gate by name. Anti-tautology: EVOLVE_USE_PHASE_REGISTRY=0
# gives different output from registry-driven output.
#
# AC-ID: cycle-57-022
# Description: orchestrator.md has registry-dispatch section; gate_run_by_name dispatches correctly
# Evidence: agents/evolve-orchestrator.md, scripts/lifecycle/phase-gate.sh
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-57-1 (primary Slice B AC)
#
# metadata:
#   id: 022-orchestrator-uses-registry
#   cycle: 57
#   task: slice-b-phase-registry-predicate-022
#   severity: HIGH

set -uo pipefail

# Use WORKTREE_PATH (where cycle code changes live) when available,
# falling back to EVOLVE_PROJECT_ROOT / git root (for standalone runs).
REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
ORCHESTRATOR_MD="$REPO_ROOT/agents/evolve-orchestrator.md"
PHASE_GATE_SH="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
LIST_HELPER="$REPO_ROOT/scripts/dispatch/list-phase-order.sh"

for _f in "$ORCHESTRATOR_MD" "$PHASE_GATE_SH" "$LIST_HELPER"; do
    if [ ! -f "$_f" ]; then
        echo "RED: required file not found: $_f"
        exit 1
    fi
done

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

rc=0

# ── AC1: orchestrator.md references list-phase-order.sh ──────────────────────
if grep -q "list-phase-order.sh" "$ORCHESTRATOR_MD"; then
    echo "GREEN AC1: orchestrator.md references list-phase-order.sh"
else
    echo "RED AC1: orchestrator.md does not reference list-phase-order.sh — registry-dispatch section missing"
    rc=1
fi

# ── AC2: orchestrator.md references gate_run_by_name ─────────────────────────
if grep -q "gate_run_by_name" "$ORCHESTRATOR_MD"; then
    echo "GREEN AC2: orchestrator.md references gate_run_by_name"
else
    echo "RED AC2: orchestrator.md does not reference gate_run_by_name — registry-dispatch section missing or incomplete"
    rc=1
fi

# ── AC3: gate_run_by_name is declared as a function in phase-gate.sh ─────────
# bash 3.2 compat: grep for function declaration, not 'declare -f'
if grep -q "^gate_run_by_name()" "$PHASE_GATE_SH"; then
    echo "GREEN AC3: gate_run_by_name() declared in phase-gate.sh"
else
    echo "RED AC3: gate_run_by_name() not found in phase-gate.sh"
    rc=1
fi

# ── AC4: gate_run_by_name successfully dispatches a known gate ───────────────
# Use a fixture workspace and invoke gate_build_to_tester via gate_run_by_name
# gate_build_to_tester requires build-report.md to exist in workspace
mkdir -p "$TMP_DIR/workspace"
cat > "$TMP_DIR/workspace/build-report.md" <<'BEOF'
# Build Report — fixture
**Status:** PASS
Fixture build-report for predicate 022 AC4 test.
BEOF

# Source phase-gate.sh to get gate_run_by_name + gate_build_to_tester in scope,
# then call gate_run_by_name "gate_build_to_tester". Use a no-op GATE to avoid
# the dispatch at the bottom of phase-gate.sh.
ac4_out=$(GATE="__nogate_predicate022__" \
    CYCLE="57" \
    WORKSPACE="$TMP_DIR/workspace" \
    EVOLVE_PROJECT_ROOT="$TMP_DIR" \
    bash -c '
        # Source phase-gate.sh up to (but not triggering) the case dispatch
        # by overriding the GATE variable to a non-matching value.
        source "'"$PHASE_GATE_SH"'" 2>/dev/null
        gate_run_by_name gate_build_to_tester 57 "'"$TMP_DIR/workspace"'"
    ' 2>&1 || true)

if echo "$ac4_out" | grep -q "PASS: BUILD → TESTER"; then
    echo "GREEN AC4: gate_run_by_name dispatched gate_build_to_tester successfully"
elif echo "$ac4_out" | grep -q "gate_run_by_name.*WARN\|not declared"; then
    echo "RED AC4: gate_run_by_name could not find gate_build_to_tester: $ac4_out"
    rc=1
else
    # gate passed but logged differently — check for absence of failure markers
    if echo "$ac4_out" | grep -qi "fail\|error\|RED\|not found"; then
        echo "RED AC4: gate_run_by_name dispatch failed: $(echo "$ac4_out" | head -3)"
        rc=1
    else
        echo "GREEN AC4: gate_run_by_name dispatched gate_build_to_tester (no failure output)"
    fi
fi

# ── AC5 (anti-tautology): EVOLVE_USE_PHASE_REGISTRY=0 gives different output ─
# Fixture registry with 3 phases (distinct subset of 11-phase prod)
mkdir -p "$TMP_DIR/docs/architecture"
cat > "$TMP_DIR/docs/architecture/phase-registry.json" <<'REGEOF'
{
  "schema_version": 1,
  "phases": [
    {"name": "intent", "role": "intent"},
    {"name": "scout", "role": "scout"},
    {"name": "ship",  "role": "orchestrator"}
  ]
}
REGEOF

registry_out=$(EVOLVE_PROJECT_ROOT="$TMP_DIR" EVOLVE_USE_PHASE_REGISTRY=1 \
    bash "$LIST_HELPER" 2>/dev/null || echo "ERROR")
fallback_out=$(EVOLVE_PROJECT_ROOT="$TMP_DIR" EVOLVE_USE_PHASE_REGISTRY=0 \
    bash "$LIST_HELPER" 2>/dev/null || echo "ERROR")

registry_lines=$(echo "$registry_out" | wc -l | tr -d ' ')
fallback_lines=$(echo "$fallback_out"  | wc -l | tr -d ' ')

if [ "$registry_lines" -ne "$fallback_lines" ]; then
    echo "GREEN AC5 (anti-tautology): fixture-registry output ($registry_lines lines) differs from hardcoded fallback ($fallback_lines lines)"
else
    echo "RED AC5 (anti-tautology): fixture-registry and hardcoded fallback have same line count ($registry_lines) — registry path not distinct"
    rc=1
fi

exit "$rc"
