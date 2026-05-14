#!/usr/bin/env bash
# ACS predicate 002 — cycle 48
# Verifies that memo.json max_turns is 15 (raised from 10, T2 fix).
#
# metadata:
#   id: 002-memo-max-turns-15
#   cycle: 48
#   task: T2
#   severity: HIGH
set -uo pipefail

MEMO_JSON="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}/.evolve/profiles/memo.json"

actual=$(jq -r '.max_turns // empty' "$MEMO_JSON" 2>/dev/null)

if [ "$actual" = "15" ]; then
    echo "GREEN: memo.json max_turns=15"
    exit 0
else
    echo "RED: memo.json max_turns='$actual' (expected 15)"
    exit 1
fi
