#!/usr/bin/env bash
# ACS predicate 001 — cycle 68
# Asserts that .evolve/profiles/intent.json:max_turns is >= 8, raised
# from the cycle-67 value of 4 which caused a recurring false-positive
# turn-overrun WARN in every cycle (intent persona legitimately uses
# ~10 turns to author structured intent.md with anchors + challenged
# premises + acceptance_checks).
#
# AC-ID: cycle-68-001
# Description: intent-max-turns-raised
# Evidence: jq-reads .evolve/profiles/intent.json:max_turns and asserts
#           it is >= 8 (was 4) and <= 32 (sanity ceiling). Resolves
#           the recurring turn-overrun WARN observed in cycle-68 intent.
# Author: builder
# Created: 2026-05-17T00:00:00Z
# Acceptance-of: scout-report.md cycle-68 Task 1
#
# metadata:
#   id: 001-intent-max-turns-raised
#   cycle: 68
#   task: raise-intent-ceiling
#   severity: MEDIUM

set -uo pipefail

# Resolve repo root from this script's own location: predicates live at
# <root>/acs/cycle-N/<id>.sh — climb up two dirs to find the project root
# whose worktree state we're validating. We deliberately avoid trusting
# EVOLVE_PROJECT_ROOT here because during audit the predicate must see
# the worktree's edits, not the as-yet-unmerged main-repo state.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
PROFILE="$REPO_ROOT/.evolve/profiles/intent.json"

if [ ! -f "$PROFILE" ]; then
    echo "RED: intent.json profile not found at $PROFILE" >&2
    exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "RED: jq not available — cannot validate profile" >&2
    exit 1
fi

max_turns=$(jq -r '.max_turns // 0' "$PROFILE")

if ! [[ "$max_turns" =~ ^[0-9]+$ ]]; then
    echo "RED: max_turns is not numeric: '$max_turns'" >&2
    exit 1
fi

if [ "$max_turns" -lt 8 ]; then
    echo "RED: intent.json:max_turns=$max_turns is below the required minimum of 8" >&2
    exit 1
fi

# Sanity ceiling — anything above 32 would be unjustified inflation.
if [ "$max_turns" -gt 32 ]; then
    echo "RED: intent.json:max_turns=$max_turns is unjustifiably high (> 32)" >&2
    exit 1
fi

echo "GREEN: intent.json:max_turns=$max_turns (>= 8, <= 32)"
exit 0
