#!/usr/bin/env bash
# ACS predicate 003 — cycle 48
# Verifies that evolve-auditor.md STOP CRITERION contains the hard turn-count
# gate (T2 fix for auditor turn-overrun).
#
# metadata:
#   id: 003-auditor-stop-criterion-hard-gate
#   cycle: 48
#   task: T2
#   severity: HIGH
set -uo pipefail

AUDITOR_MD="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}/agents/evolve-auditor.md"

if grep -q "turn count > 30" "$AUDITOR_MD" 2>/dev/null; then
    echo "GREEN: evolve-auditor.md contains 'turn count > 30' hard-gate"
    exit 0
else
    echo "RED: evolve-auditor.md missing 'turn count > 30' hard-gate in STOP CRITERION"
    exit 1
fi
