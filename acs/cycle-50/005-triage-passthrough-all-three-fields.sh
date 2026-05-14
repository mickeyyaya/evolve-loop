#!/usr/bin/env bash
# ACS predicate 005 — cycle 50
# Triage passthrough paragraph preserves all three research-cache fields
#
# AC-ID: cycle-50-005
# Description: evolve-triage.md passthrough paragraph mentions research_pointer, research_fingerprint, and research_cycle
# Evidence: agents/evolve-triage.md:50
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-5
#
# metadata:
#   id: 005-triage-passthrough-all-three-fields
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
TRIAGE="$REPO_ROOT/agents/evolve-triage.md"
[ -f "$TRIAGE" ] || { echo "RED: $TRIAGE not found"; exit 1; }

rc=0

# AC1-AC3: All three passthrough fields present
for field in "research_pointer" "research_fingerprint" "research_cycle"; do
    if ! grep -q "$field" "$TRIAGE"; then
        echo "RED: '$field' not found in evolve-triage.md (passthrough field missing)"
        rc=1
    else
        echo "GREEN: '$field' found in evolve-triage.md passthrough contract"
    fi
done

exit $rc
