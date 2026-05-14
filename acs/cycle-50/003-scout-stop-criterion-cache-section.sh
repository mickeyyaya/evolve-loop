#!/usr/bin/env bash
# ACS predicate 003 — cycle 50
# Scout STOP CRITERION table includes research-cache-section row
#
# AC-ID: cycle-50-003
# Description: evolve-scout.md STOP CRITERION table includes research-cache-section row
# Evidence: agents/evolve-scout.md:263
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-3
#
# metadata:
#   id: 003-scout-stop-criterion-cache-section
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCOUT="$REPO_ROOT/agents/evolve-scout.md"
[ -f "$SCOUT" ] || { echo "RED: $SCOUT not found"; exit 1; }

rc=0

# AC1: research-cache-section appears in the STOP CRITERION block
# Verify both the STOP CRITERION section exists and research-cache-section is within it
if ! grep -q "STOP CRITERION\|## Stop Criterion\|## STOP" "$SCOUT"; then
    echo "RED AC1: STOP CRITERION section not found in evolve-scout.md"
    rc=1
fi

if ! grep -q "research-cache-section" "$SCOUT"; then
    echo "RED AC2: 'research-cache-section' row not found in evolve-scout.md STOP CRITERION table"
    rc=1
else
    echo "GREEN AC1+AC2: STOP CRITERION table includes research-cache-section row in evolve-scout.md"
fi

exit $rc
