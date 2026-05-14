#!/usr/bin/env bash
# AC5: acs/cycle-46/010-015.sh files have cycle=46 (not cycle=47) in metadata
# predicate: T4 — metadata header correctness
# metadata: cycle=47 task=T4 ac=AC5 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
ACS_DIR="$REPO_ROOT/acs/cycle-46"
[ -d "$ACS_DIR" ] || { echo "FAIL: $ACS_DIR not found"; exit 1; }
bad_count=0
for f in "$ACS_DIR"/01[0-5]-*.sh; do
    [ -f "$f" ] || continue
    if grep -q "cycle=47" "$f" 2>/dev/null; then
        echo "FAIL: $(basename "$f") still has cycle=47"
        bad_count=$((bad_count + 1))
    fi
done
[ "$bad_count" -eq 0 ] || { echo "FAIL: $bad_count files still have cycle=47 in acs/cycle-46/"; exit 1; }
echo "PASS: all acs/cycle-46/010-015 files have cycle=46 (T4)"
