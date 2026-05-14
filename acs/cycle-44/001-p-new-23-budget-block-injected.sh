#!/usr/bin/env bash
# AC-ID:         cycle-44-001
# Description:   role-context-builder.sh emits ## Budget block for roles with turn_budget_hint
# Evidence:      scripts/lifecycle/role-context-builder.sh (emit_budget_hint function)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T1 (P-NEW-23)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/scripts/lifecycle/role-context-builder.sh"

[ -f "$FILE" ] || { echo "FAIL: role-context-builder.sh not found"; exit 1; }

# emit_budget_hint function must exist
grep -q "emit_budget_hint" "$FILE" || { echo "FAIL: emit_budget_hint function not found in role-context-builder.sh"; exit 1; }

# Must inject ## Budget block
grep -q "## Budget" "$FILE" || { echo "FAIL: ## Budget block injection not found in role-context-builder.sh"; exit 1; }

# Must read turn_budget_hint from profile
grep -q "turn_budget_hint" "$FILE" || { echo "FAIL: turn_budget_hint not referenced in role-context-builder.sh"; exit 1; }

# Must call emit_budget_hint from header_block
HEADER_BLOCK_CALLS=$(awk '/^header_block\(\)/,/^}/' "$FILE" | grep -c "emit_budget_hint" || true)
[ "$HEADER_BLOCK_CALLS" -ge 1 ] || { echo "FAIL: emit_budget_hint not called from header_block() (got $HEADER_BLOCK_CALLS calls)"; exit 1; }

echo "PASS: role-context-builder.sh has emit_budget_hint wired into header_block with ## Budget injection"
exit 0
