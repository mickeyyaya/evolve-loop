#!/usr/bin/env bash
# ACS predicate: 011 — state.json:lastCycleNumber == 47 post-ship
# cycle: 47
# task: T3 (end-to-end validation of counter-advance fix)
# severity: MEDIUM
# NOTE: This predicate reads state.json after ship completes. It verifies that the
# T1 counter-advance fix works end-to-end: cycle-47 ship must leave lastCycleNumber=47.
set -uo pipefail

STATE_FILE="${EVOLVE_PROJECT_ROOT:-.}/.evolve/state.json"

if [ ! -f "$STATE_FILE" ]; then
    echo "FAIL: state.json not found at $STATE_FILE" >&2
    exit 1
fi

_actual=$(jq -r '.lastCycleNumber // empty' "$STATE_FILE" 2>/dev/null || echo "")
if [ -z "$_actual" ] || [ "$_actual" = "null" ]; then
    echo "FAIL: state.json:lastCycleNumber is absent or null" >&2
    exit 1
fi

if [ "$_actual" != "47" ]; then
    echo "FAIL: state.json:lastCycleNumber=$_actual, expected 47" >&2
    exit 1
fi

echo "PASS: state.json:lastCycleNumber=47"
exit 0
