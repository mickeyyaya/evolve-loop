#!/usr/bin/env bash
# AC4: evolve-scout.md has parallel tool-call batching guidance
# predicate: T3 — P-NEW-29 guidance present in scout persona
# metadata: cycle=47 task=T3 ac=AC4 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/agents/evolve-scout.md"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "Parallel Tool-Call Batching" "$TARGET" || { echo "FAIL: 'Parallel Tool-Call Batching' section not in evolve-scout.md"; exit 1; }
echo "PASS: evolve-scout.md has parallel batching guidance (T3)"
