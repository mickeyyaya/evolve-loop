#!/usr/bin/env bash
# ACS predicate 004 — cycle 50
# Builder Step 2.5 references research_pointer and per-task-cache source
#
# AC-ID: cycle-50-004
# Description: evolve-builder.md Step 2.5 contains research_pointer and Research Source: per-task-cache
# Evidence: agents/evolve-builder.md:133-134
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-4
#
# metadata:
#   id: 004-builder-step-2-5-research-pointer
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
BUILDER="$REPO_ROOT/agents/evolve-builder.md"
[ -f "$BUILDER" ] || { echo "RED: $BUILDER not found"; exit 1; }

rc=0

# AC1: research_pointer field referenced in builder step 2.5 area
if ! grep -q "research_pointer" "$BUILDER"; then
    echo "RED AC1: 'research_pointer' field not found in evolve-builder.md (cache integration missing)"
    rc=1
else
    echo "GREEN AC1: research_pointer referenced in evolve-builder.md"
fi

# AC2: Research Source: per-task-cache label is present
if ! grep -q "Research Source: per-task-cache" "$BUILDER"; then
    echo "RED AC2: 'Research Source: per-task-cache' not found in evolve-builder.md (cache source label missing)"
    rc=1
else
    echo "GREEN AC2: 'Research Source: per-task-cache' found in evolve-builder.md"
fi

exit $rc
