#!/usr/bin/env bash
# AC6: token-reduction-roadmap.md has P-NEW-30 entry (TACO)
# predicate: T4 — P-NEW-30 roadmap entry present
# metadata: cycle=47 task=T4 ac=AC6 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "P-NEW-30" "$TARGET" || { echo "FAIL: P-NEW-30 not in roadmap"; exit 1; }
grep -q "TACO" "$TARGET" || { echo "FAIL: TACO not in roadmap"; exit 1; }
grep -q "2604.19572" "$TARGET" || { echo "FAIL: arXiv:2604.19572 not in roadmap"; exit 1; }
echo "PASS: roadmap has P-NEW-30 TACO entry (T4)"
