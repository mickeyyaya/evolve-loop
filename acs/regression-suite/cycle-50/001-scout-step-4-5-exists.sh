#!/usr/bin/env bash
# ACS predicate 001 — cycle 50
# Scout Step 4.5 Per-Task Research Cache Lookup exists with all six exit codes
#
# AC-ID: cycle-50-001
# Description: evolve-scout.md contains Step 4.5 header AND all six cache-check exit codes
# Evidence: agents/evolve-scout.md:74
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-1
#
# metadata:
#   id: 001-scout-step-4-5-exists
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCOUT="$REPO_ROOT/agents/evolve-scout.md"
[ -f "$SCOUT" ] || { echo "RED: $SCOUT not found"; exit 1; }

rc=0

# AC1: Step 4.5 header is present
if ! grep -q "### 4\.5\." "$SCOUT"; then
    echo "RED AC1: '### 4.5.' header not found in evolve-scout.md (Step 4.5 Per-Task Research Cache Lookup missing)"
    rc=1
else
    echo "GREEN AC1: Step 4.5 header found in evolve-scout.md"
fi

# AC2-AC7: All six exit codes documented
for code in "0 (HIT)" "10 (STALE)" "20 (MISS)" "30 (INVALIDATED)" "40 (NO_ENTRY)" "50 (DISABLED)"; do
    if ! grep -q "$code" "$SCOUT"; then
        echo "RED AC2: exit code '$code' not found in evolve-scout.md Step 4.5 block"
        rc=1
    else
        echo "GREEN AC2: exit code '$code' documented in evolve-scout.md"
    fi
done

exit $rc
