#!/usr/bin/env bash
# ACS predicate 021 — cycle 55
# scripts/dispatch/list-phase-order.sh emits phase names from the registry when
# EVOLVE_USE_PHASE_REGISTRY=1 (default), and falls back to the hardcoded order
# when the registry is absent or EVOLVE_USE_PHASE_REGISTRY=0. Anti-tautology:
# fixture-registry output must differ from hardcoded-order output (length check).
#
# AC-ID: cycle-55-021
# Description: list-phase-order.sh reads registry or falls back correctly
# Evidence: scripts/dispatch/list-phase-order.sh
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-6
#
# metadata:
#   id: 021-orchestrator-reads-registry-not-narrative
#   cycle: 55
#   task: slice-b-phase-registry
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
LIST_HELPER="$REPO_ROOT/scripts/dispatch/list-phase-order.sh"

if [ ! -f "$LIST_HELPER" ]; then
    echo "RED: list-phase-order.sh not found at $LIST_HELPER"
    exit 1
fi

# ── Setup ─────────────────────────────────────────────────────────────────────
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Fixture registry with exactly 3 phases (a minimal subset, distinct from 11-phase prod)
mkdir -p "$TMP_DIR/docs/architecture"
cat > "$TMP_DIR/docs/architecture/phase-registry.json" <<'REGEOF'
{
  "schema_version": 1,
  "phases": [
    {"name": "intent", "role": "intent"},
    {"name": "scout", "role": "scout"},
    {"name": "ship", "role": "orchestrator"}
  ]
}
REGEOF

rc=0

# ── AC1: with registry, helper emits phase names from registry ────────────────
ac1_output=$(EVOLVE_PROJECT_ROOT="$TMP_DIR" EVOLVE_USE_PHASE_REGISTRY=1 \
    bash "$LIST_HELPER" 2>/dev/null || echo "ERROR")

if [ "$ac1_output" = "intent
scout
ship" ]; then
    echo "GREEN AC1: fixture registry (3-phase) → output matches expected order"
else
    echo "RED AC1: fixture registry output was: $(echo "$ac1_output" | tr '\n' '|')"
    rc=1
fi

# ── AC2: with EVOLVE_USE_PHASE_REGISTRY=0, falls back to hardcoded order ──────
ac2_output=$(EVOLVE_PROJECT_ROOT="$TMP_DIR" EVOLVE_USE_PHASE_REGISTRY=0 \
    bash "$LIST_HELPER" 2>/dev/null || echo "ERROR")

# Hardcoded order must contain at least 'scout' and 'build'
if [[ "$ac2_output" == *"scout"* ]] && [[ "$ac2_output" == *"build"* ]]; then
    echo "GREEN AC2: EVOLVE_USE_PHASE_REGISTRY=0 → hardcoded fallback contains scout+build"
else
    echo "RED AC2: EVOLVE_USE_PHASE_REGISTRY=0 fallback output missing scout or build: $(echo "$ac2_output" | tr '\n' '|')"
    rc=1
fi

# ── AC3: with registry absent and EVOLVE_USE_PHASE_REGISTRY=1, falls back ─────
# Use a TMP_DIR with no registry file
EMPTY_DIR=$(mktemp -d)
trap 'rm -rf "$EMPTY_DIR"' EXIT
ac3_output=$(EVOLVE_PROJECT_ROOT="$EMPTY_DIR" EVOLVE_USE_PHASE_REGISTRY=1 \
    bash "$LIST_HELPER" 2>/dev/null || echo "ERROR")

if [[ "$ac3_output" == *"scout"* ]] && [[ "$ac3_output" == *"build"* ]]; then
    echo "GREEN AC3: registry absent → hardcoded fallback contains scout+build"
else
    echo "RED AC3: registry absent fallback output missing scout or build: $(echo "$ac3_output" | tr '\n' '|')"
    rc=1
fi

# ── Anti-tautology: AC1 output length != AC2 output length ───────────────────
ac1_lines=$(echo "$ac1_output" | wc -l | tr -d ' ')
ac2_lines=$(echo "$ac2_output" | wc -l | tr -d ' ')
if [ "$ac1_lines" -ne "$ac2_lines" ]; then
    echo "GREEN anti-tautology: fixture output ($ac1_lines lines) != hardcoded output ($ac2_lines lines)"
else
    echo "RED anti-tautology: fixture output and hardcoded output have same line count ($ac1_lines) — registry not actually consulted"
    rc=1
fi

exit "$rc"
