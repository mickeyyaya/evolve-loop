#!/usr/bin/env bash
# AC-ID:         cycle-44-005
# Description:   evolve-orchestrator.md Phase Loop 5b/5c both call gate_retrospective_to_complete
# Evidence:      agents/evolve-orchestrator.md (Phase Loop code block)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T3 (D-1 retro gate wired)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/agents/evolve-orchestrator.md"

[ -f "$FILE" ] || { echo "FAIL: evolve-orchestrator.md not found"; exit 1; }

COUNT=$(grep -c "gate_retrospective_to_complete" "$FILE" || true)
[ "$COUNT" -ge 2 ] || { echo "FAIL: gate_retrospective_to_complete appears fewer than 2 times (got $COUNT); must be in both Phase Loop 5b and 5c"; exit 1; }

echo "PASS: gate_retrospective_to_complete appears $COUNT times in evolve-orchestrator.md (>=2 required)"
exit 0
