#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that evolve-triage.md contains the operator-queue priority floor rule.
set -uo pipefail

AGENT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/agents/evolve-triage.md"

[ -f "$AGENT" ] || { echo "FAIL: evolve-triage.md not found at $AGENT" >&2; exit 1; }

if ! grep -qF 'Operator-queue priority floor' "$AGENT"; then
    echo "FAIL: evolve-triage.md missing Operator-queue priority floor rule" >&2
    exit 1
fi

if ! grep -q 'at least one.*top_n.*slot\|top_n.*slot.*reserved' "$AGENT"; then
    echo "FAIL: evolve-triage.md missing mandatory top_n slot reservation for HIGH operator-queued items" >&2
    exit 1
fi

echo "PASS: evolve-triage.md contains operator-queue priority floor rule"
exit 0
