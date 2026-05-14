#!/usr/bin/env bash
# AC7: roadmap marks P-NEW-22 as DONE (cycle 47) in status table
# predicate: T1 — roadmap status table updated
# metadata: cycle=47 task=T1 ac=AC7 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "P-NEW-22.*DONE.*cycle 47" "$TARGET" || { echo "FAIL: P-NEW-22 not marked DONE (cycle 47) in roadmap"; exit 1; }
echo "PASS: roadmap shows P-NEW-22 DONE (cycle 47) (T1)"
