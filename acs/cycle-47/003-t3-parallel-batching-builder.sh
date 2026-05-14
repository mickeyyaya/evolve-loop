#!/usr/bin/env bash
# AC3: evolve-builder.md has Parallel Tool-Call Batching section with >=2 examples
# predicate: T3 — P-NEW-29 guidance present in builder persona
# metadata: cycle=47 task=T3 ac=AC3 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/agents/evolve-builder.md"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "Parallel Tool-Call Batching" "$TARGET" || { echo "FAIL: 'Parallel Tool-Call Batching' section not in evolve-builder.md"; exit 1; }
# Verify at least 2 before/after example rows (SLOW / FAST pattern)
count=$(grep -c "SLOW\|FAST" "$TARGET" 2>/dev/null || echo 0)
[ "$count" -ge 2 ] || { echo "FAIL: fewer than 2 before/after examples in evolve-builder.md (found $count)"; exit 1; }
echo "PASS: evolve-builder.md has parallel batching guidance with $count SLOW/FAST markers (T3)"
