#!/usr/bin/env bash
# AC1: claude.sh reads schema_filter_enabled and auto-injects --strict-mcp-config
# predicate: P-NEW-22 Phase 2 — dispatch-layer enforcement present in claude.sh
# metadata: cycle=47 task=T1 ac=AC1 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/scripts/cli_adapters/claude.sh"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "SCHEMA_FILTER_ENABLED" "$TARGET" || { echo "FAIL: SCHEMA_FILTER_ENABLED not in claude.sh"; exit 1; }
grep -q "schema_filter_enabled" "$TARGET" || { echo "FAIL: schema_filter_enabled jq read not in claude.sh"; exit 1; }
grep -q "strict-mcp-config" "$TARGET" || { echo "FAIL: strict-mcp-config injection not in claude.sh"; exit 1; }
echo "PASS: claude.sh has P-NEW-22 Phase 2 schema_filter_enabled enforcement"
