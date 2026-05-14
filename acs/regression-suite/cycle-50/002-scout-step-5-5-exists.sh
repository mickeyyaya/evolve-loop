#!/usr/bin/env bash
# ACS predicate 002 — cycle 50
# Scout Step 5.5 Stage Per-Task Research to Cache Staging exists in evolve-scout.md
#
# AC-ID: cycle-50-002
# Description: evolve-scout.md contains Step 5.5 header AND research-cache-staging worker path
# Evidence: agents/evolve-scout.md:102
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-2
#
# metadata:
#   id: 002-scout-step-5-5-exists
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCOUT="$REPO_ROOT/agents/evolve-scout.md"
[ -f "$SCOUT" ] || { echo "RED: $SCOUT not found"; exit 1; }

rc=0

# AC1: Step 5.5 header is present
if ! grep -q "### 5\.5\." "$SCOUT"; then
    echo "RED AC1: '### 5.5.' header not found in evolve-scout.md (Step 5.5 Stage Research missing)"
    rc=1
else
    echo "GREEN AC1: Step 5.5 header found in evolve-scout.md"
fi

# AC2: research-cache-staging worker path referenced
if ! grep -q "research-cache-staging" "$SCOUT"; then
    echo "RED AC2: 'research-cache-staging' path not found in evolve-scout.md (cache staging worker dir missing)"
    rc=1
else
    echo "GREEN AC2: research-cache-staging worker path referenced in evolve-scout.md"
fi

exit $rc
