#!/usr/bin/env bash
# AC8: abnormal-event-capture.md has ship-refused tree-drift recovery pattern
# predicate: T4 — ship-refused recovery documented
# metadata: cycle=47 task=T4 ac=AC8 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/docs/architecture/abnormal-event-capture.md"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "ship-refused.*tree-drift\|tree-drift.*ship-refused" "$TARGET" || { echo "FAIL: ship-refused tree-drift pattern not documented in abnormal-event-capture.md"; exit 1; }
echo "PASS: ship-refused tree-drift recovery documented (T4)"
