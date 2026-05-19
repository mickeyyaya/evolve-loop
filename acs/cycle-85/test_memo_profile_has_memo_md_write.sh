#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that memo.json allowed_tools includes Write and Edit for memo.md
set -uo pipefail

PROFILE="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/.evolve/profiles/memo.json"

[ -f "$PROFILE" ] || { echo "FAIL: memo.json not found at $PROFILE" >&2; exit 1; }

if ! grep -qF '"Write(.evolve/runs/cycle-*/memo.md)"' "$PROFILE"; then
    echo "FAIL: memo.json missing Write(.evolve/runs/cycle-*/memo.md) in allowed_tools" >&2
    exit 1
fi

if ! grep -qF '"Edit(.evolve/runs/cycle-*/memo.md)"' "$PROFILE"; then
    echo "FAIL: memo.json missing Edit(.evolve/runs/cycle-*/memo.md) in allowed_tools" >&2
    exit 1
fi

echo "PASS: memo.json allowed_tools contains Write and Edit for memo.md"
exit 0
