#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that evolve-orchestrator.md contains the completed_phases closure-mode check.
set -uo pipefail

AGENT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/agents/evolve-orchestrator.md"

[ -f "$AGENT" ] || { echo "FAIL: evolve-orchestrator.md not found at $AGENT" >&2; exit 1; }

if ! grep -qF 'completed_phases' "$AGENT"; then
    echo "FAIL: evolve-orchestrator.md missing completed_phases closure-mode check" >&2
    exit 1
fi

if ! grep -q 'Skip directly to ship phase\|skip.*ship phase\|skip directly to ship' "$AGENT"; then
    echo "FAIL: evolve-orchestrator.md missing skip-to-ship directive for closure cycles" >&2
    exit 1
fi

echo "PASS: evolve-orchestrator.md contains completed_phases closure-mode check"
exit 0
