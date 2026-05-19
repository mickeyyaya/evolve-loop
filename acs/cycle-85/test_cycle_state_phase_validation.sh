#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that cycle-state.sh validates phase names in cycle_state_advance().
set -uo pipefail

SCRIPT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/scripts/lifecycle/cycle-state.sh"

[ -f "$SCRIPT" ] || { echo "FAIL: cycle-state.sh not found at $SCRIPT" >&2; exit 1; }

# Must contain the case statement with known phases
if ! grep -q 'calibrate|intent|research|discover|triage' "$SCRIPT"; then
    echo "FAIL: cycle-state.sh missing phase validation case statement" >&2
    exit 1
fi

# Must contain the unknown phase error + return 2
if ! grep -qF "ERROR: unknown phase" "$SCRIPT"; then
    echo "FAIL: cycle-state.sh missing 'ERROR: unknown phase' error message" >&2
    exit 1
fi

if ! grep -qF 'return 2' "$SCRIPT"; then
    echo "FAIL: cycle-state.sh missing return 2 for invalid phase" >&2
    exit 1
fi

echo "PASS: cycle-state.sh has phase name validation in cycle_state_advance()"
exit 0
