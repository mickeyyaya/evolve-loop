#!/usr/bin/env bash
# AC8: grep -q "P-NEW-27" docs/architecture/token-reduction-roadmap.md passes
# metadata: cycle=46 task=T2b ac=AC8 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
ROADMAP="$PROJECT_ROOT/docs/architecture/token-reduction-roadmap.md"

if ! grep -q "P-NEW-27" "$ROADMAP"; then
    echo "FAIL: P-NEW-27 not found in token-reduction-roadmap.md" >&2
    exit 1
fi

# Verify it's marked as DONE
if ! grep "P-NEW-27" "$ROADMAP" | grep -qi "DONE\|done"; then
    echo "FAIL: P-NEW-27 in roadmap not marked as DONE (cycle 46)" >&2
    exit 1
fi

echo "PASS: AC8 — P-NEW-27 entry present in token-reduction-roadmap.md"
exit 0
