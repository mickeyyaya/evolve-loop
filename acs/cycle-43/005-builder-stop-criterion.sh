#!/usr/bin/env bash
# AC-ID:         cycle-43-005
# Description:   evolve-builder.md contains STOP CRITERION section with >=3 completion gates
# Evidence:      agents/evolve-builder.md (## STOP CRITERION section)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T2 (A1)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/agents/evolve-builder.md"

[ -f "$FILE" ] || { echo "FAIL: evolve-builder.md not found"; exit 1; }

# STOP CRITERION section must exist
grep -q "## STOP CRITERION" "$FILE" || { echo "FAIL: ## STOP CRITERION section not found in evolve-builder.md"; exit 1; }

# Must have at least 3 named completion gates
GATE_COUNT=$(grep -c "worktree-verified\|implementation-complete\|self-verify-passed\|report-written" "$FILE" || true)
[ "$GATE_COUNT" -ge 3 ] || { echo "FAIL: fewer than 3 named completion gates found (got $GATE_COUNT)"; exit 1; }

# Must have banned post-report patterns
grep -q "Banned Post-Report\|banned.*post.*report\|post-report" "$FILE" || { echo "FAIL: banned post-report patterns section not found"; exit 1; }

echo "PASS: evolve-builder.md has STOP CRITERION with >=3 completion gates and banned post-report patterns"
exit 0
