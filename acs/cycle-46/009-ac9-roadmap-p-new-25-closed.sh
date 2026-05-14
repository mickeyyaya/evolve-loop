#!/usr/bin/env bash
# AC9: grep -q "P-NEW-25.*CLOSED" docs/architecture/token-reduction-roadmap.md passes
# metadata: cycle=46 task=T2b ac=AC9 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
ROADMAP="$PROJECT_ROOT/docs/architecture/token-reduction-roadmap.md"

if ! grep -q "P-NEW-25" "$ROADMAP"; then
    echo "FAIL: P-NEW-25 not found in token-reduction-roadmap.md" >&2
    exit 1
fi

if ! grep "P-NEW-25" "$ROADMAP" | grep -qi "CLOSED\|closed"; then
    echo "FAIL: P-NEW-25 not marked CLOSED in token-reduction-roadmap.md" >&2
    grep "P-NEW-25" "$ROADMAP" >&2
    exit 1
fi

echo "PASS: AC9 — P-NEW-25 marked CLOSED in token-reduction-roadmap.md"
exit 0
