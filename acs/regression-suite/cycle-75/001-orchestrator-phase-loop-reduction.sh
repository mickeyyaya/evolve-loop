#!/bin/bash
# ACS Cycle-75 AC1-AC3: orchestrator Phase Loop body cold-move
# AC4 (anti-tautology) is a revert-test; see scout-report.md for rationale.
set -uo pipefail

ORCHESTRATOR="agents/evolve-orchestrator.md"
REFERENCE="agents/evolve-orchestrator-reference.md"

# AC1: orchestrator.md line count <= 291 (baseline 341, must remove >= 50 lines)
line_count=$(wc -l < "$ORCHESTRATOR")
if [ "$line_count" -gt 291 ]; then
    echo "FAIL AC1: orchestrator.md has $line_count lines (expected <= 291)" >&2
    exit 1
fi

# AC2: canonical section exists in reference doc
if ! grep -q '^## Section: legacy-phase-loop' "$REFERENCE"; then
    echo "FAIL AC2: '## Section: legacy-phase-loop' not found in $REFERENCE" >&2
    exit 1
fi

# AC3: Phase Loop heading preserved in orchestrator.md (regression-suite/cycle-42 AC3)
if ! grep -q '^## Phase Loop' "$ORCHESTRATOR"; then
    echo "FAIL AC3: '## Phase Loop' heading missing from $ORCHESTRATOR" >&2
    exit 1
fi

echo "PASS: AC1 (lines=$line_count <= 291), AC2 (legacy-phase-loop in reference), AC3 (Phase Loop heading preserved)"
