#!/usr/bin/env bash
# AC-ID:         cycle-43-006
# Description:   token-reduction-roadmap.md contains P-NEW-21 and P-NEW-22 entries
# Evidence:      docs/architecture/token-reduction-roadmap.md
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T4 (A1)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"

[ -f "$FILE" ] || { echo "FAIL: token-reduction-roadmap.md not found"; exit 1; }

# P-NEW-21 must exist (AgentDiet trajectory compression)
grep -q "P-NEW-21" "$FILE" || { echo "FAIL: P-NEW-21 not found in roadmap"; exit 1; }

# P-NEW-22 must exist (Selective MCP tool-schema)
grep -q "P-NEW-22" "$FILE" || { echo "FAIL: P-NEW-22 not found in roadmap"; exit 1; }

# P-NEW-23 must exist (token-budget-aware hints)
grep -q "P-NEW-23" "$FILE" || { echo "FAIL: P-NEW-23 not found in roadmap"; exit 1; }

# P-NEW-20 must be DONE in summary table
grep -q "P-NEW-20.*DONE\|P-NEW-20 Builder stop-criterion.*DONE" "$FILE" || { echo "FAIL: P-NEW-20 not marked DONE in roadmap"; exit 1; }

echo "PASS: roadmap has P-NEW-20 (DONE), P-NEW-21, P-NEW-22, and P-NEW-23 entries"
exit 0
