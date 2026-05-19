#!/usr/bin/env bash
# AC-ID: cycle-86-carryover-turn-overrun-dismissed
# Verifies that `abnormal-turn-overrun-c86` is no longer present in
# `.evolve/state.json:carryoverTodos[]` after Builder dismissal.
#
# Rationale (Scout-confirmed, Triage-gated):
#   The turn-overrun event is real (cycle-86 intent phase, 16/10 turns,
#   WARN-level) but is not a code defect — it is a class-mismatch for
#   the 10-turn ceiling against an investigation-class (IMKI) intent.
#   Dismissal with documented rationale is the correct disposition.
#
# Pre-Builder state: RED (entry present).
# Post-Builder state: GREEN (entry absent).
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
STATE_FILE="$REPO_ROOT/.evolve/state.json"
TARGET_ID="abnormal-turn-overrun-c86"

if [ ! -f "$STATE_FILE" ]; then
  echo "RED cycle-86-carryover-turn-overrun-dismissed: state.json not found at $STATE_FILE" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "RED cycle-86-carryover-turn-overrun-dismissed: jq not on PATH" >&2
  exit 1
fi

count=$(jq --arg id "$TARGET_ID" \
  '[.carryoverTodos // [] | .[] | select(.id == $id)] | length' \
  "$STATE_FILE" 2>/dev/null)

if [ -z "$count" ]; then
  echo "RED cycle-86-carryover-turn-overrun-dismissed: jq parse failed on $STATE_FILE" >&2
  exit 1
fi

if [ "$count" -ne 0 ]; then
  echo "RED cycle-86-carryover-turn-overrun-dismissed: $TARGET_ID still present ($count match(es))" >&2
  exit 1
fi

echo "GREEN cycle-86-carryover-turn-overrun-dismissed: $TARGET_ID absent from carryoverTodos[]"
exit 0
