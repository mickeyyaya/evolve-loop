#!/usr/bin/env bash
# AC-ID:         cycle-43-002
# Description:   evolve-orchestrator.md Phase Loop checks MERGE_RC after merge-lesson-into-state.sh
# Evidence:      agents/evolve-orchestrator.md (Phase Loop 5b and 5c)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T3-b (A2)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/agents/evolve-orchestrator.md"

[ -f "$FILE" ] || { echo "FAIL: evolve-orchestrator.md not found"; exit 1; }

# Phase Loop must contain MERGE_RC assignment
grep -q "MERGE_RC=\$?" "$FILE" || { echo "FAIL: MERGE_RC=\$? not found in evolve-orchestrator.md"; exit 1; }

# Must check exit 2 on INTEGRITY_FAIL
grep -q "MERGE_RC -eq 2.*exit 2\|exit 2.*INTEGRITY_FAIL" "$FILE" || { echo "FAIL: exit 2 on INTEGRITY_FAIL not found"; exit 1; }

echo "PASS: evolve-orchestrator.md Phase Loop has MERGE_RC check with exit 2 on INTEGRITY_FAIL"
exit 0
