#!/usr/bin/env bash
# ACS predicate 023 — cycle 58
# Verifies that check-phase-inputs.sh exits 1 and names missing artifacts when
# required inputs are absent, and exits 0 when all inputs are present.
#
# AC-ID: cycle-58-023
# Description: check-phase-inputs.sh detects missing phase inputs and reports them
# Evidence: scripts/utility/check-phase-inputs.sh
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-58-1 (check-phase-inputs utility)
#
# metadata:
#   id: 023-check-phase-inputs-detects-missing
#   cycle: 58
#   task: adr5-standalone-phase-runners
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
CHECK_INPUTS="$REPO_ROOT/scripts/utility/check-phase-inputs.sh"

if [ ! -f "$CHECK_INPUTS" ]; then
    echo "RED: check-phase-inputs.sh not found at $CHECK_INPUTS"
    exit 1
fi

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

FIXTURE_CYCLE=9999

# Create workspace dir but no scout-report.md or triage-decision.md
mkdir -p "$TMP/.evolve/runs/cycle-$FIXTURE_CYCLE"
mkdir -p "$TMP/.evolve"

rc=0

# ── AC1: exits 1 when builder inputs are missing ─────────────────────────────
# Re-run to capture exit code cleanly
set +e
EVOLVE_PROJECT_ROOT="$TMP" bash "$CHECK_INPUTS" build "$FIXTURE_CYCLE" > /dev/null 2>&1
actual_rc=$?
set -e

if [ "$actual_rc" -eq 1 ]; then
    echo "GREEN AC1: check-phase-inputs.sh exits 1 when build inputs missing"
else
    echo "RED AC1: expected exit 1 (missing inputs), got $actual_rc"
    rc=1
fi

# ── AC2: output names missing scout-report.md ─────────────────────────────────
set +e
ac2_output=$(EVOLVE_PROJECT_ROOT="$TMP" bash "$CHECK_INPUTS" build "$FIXTURE_CYCLE" 2>&1)
set -e

if printf '%s\n' "$ac2_output" | grep -qi "scout-report"; then
    echo "GREEN AC2: output names missing scout-report.md"
else
    echo "RED AC2: output does not name scout-report.md; got: $ac2_output"
    rc=1
fi

# ── AC3 (anti-tautology): exits 0 when all inputs are present ────────────────
# Populate required files for build phase: scout-report.md, triage-decision.md
# and state.json with instinctSummary + failedApproaches fields.
touch "$TMP/.evolve/runs/cycle-$FIXTURE_CYCLE/scout-report.md"
touch "$TMP/.evolve/runs/cycle-$FIXTURE_CYCLE/triage-decision.md"
printf '{"instinctSummary":"","failedApproaches":[]}\n' > "$TMP/.evolve/state.json"

set +e
EVOLVE_PROJECT_ROOT="$TMP" bash "$CHECK_INPUTS" build "$FIXTURE_CYCLE" > /dev/null 2>&1
ac3_rc=$?
set -e

if [ "$ac3_rc" -eq 0 ]; then
    echo "GREEN AC3 (anti-tautology): exits 0 when all build inputs present"
else
    ac3_out=$(EVOLVE_PROJECT_ROOT="$TMP" bash "$CHECK_INPUTS" build "$FIXTURE_CYCLE" 2>&1 || true)
    echo "RED AC3 (anti-tautology): expected exit 0 with inputs present, got $ac3_rc. Output: $ac3_out"
    rc=1
fi

# ── AC4: exits 2 for unknown phase name ───────────────────────────────────────
set +e
EVOLVE_PROJECT_ROOT="$TMP" bash "$CHECK_INPUTS" nonexistent-phase-xyz "$FIXTURE_CYCLE" > /dev/null 2>&1
ac4_rc=$?
set -e

if [ "$ac4_rc" -eq 2 ]; then
    echo "GREEN AC4: exits 2 for unknown phase name"
else
    echo "RED AC4: expected exit 2 for unknown phase, got $ac4_rc"
    rc=1
fi

exit "$rc"
