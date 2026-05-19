#!/usr/bin/env bash
# AC-ID: cycle-86-carryover-ship-refused-dismissed
# Verifies that `abnormal-ship-refused-c86` is no longer present in
# `.evolve/state.json:carryoverTodos[]` after Builder dismissal.
#
# Rationale (Scout-confirmed, Triage-gated):
#   No `ship-refused` event exists in either the global abnormal-events.jsonl
#   or the cycle-86 workspace log. The carryover is an inbox-projector
#   false-positive. Acceptance is removal of the entry.
#
# Pre-Builder state: RED (entry present).
# Post-Builder state: GREEN (entry absent).
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
STATE_FILE="$REPO_ROOT/.evolve/state.json"
TARGET_ID="abnormal-ship-refused-c86"

if [ ! -f "$STATE_FILE" ]; then
  echo "RED cycle-86-carryover-ship-refused-dismissed: state.json not found at $STATE_FILE" >&2
  exit 1
fi

# jq is the canonical reader for state.json across this repo.
if ! command -v jq >/dev/null 2>&1; then
  echo "RED cycle-86-carryover-ship-refused-dismissed: jq not on PATH" >&2
  exit 1
fi

count=$(jq --arg id "$TARGET_ID" \
  '[.carryoverTodos // [] | .[] | select(.id == $id)] | length' \
  "$STATE_FILE" 2>/dev/null)

if [ -z "$count" ]; then
  echo "RED cycle-86-carryover-ship-refused-dismissed: jq parse failed on $STATE_FILE" >&2
  exit 1
fi

if [ "$count" -ne 0 ]; then
  echo "RED cycle-86-carryover-ship-refused-dismissed: $TARGET_ID still present ($count match(es))" >&2
  exit 1
fi

echo "GREEN cycle-86-carryover-ship-refused-dismissed: $TARGET_ID absent from carryoverTodos[]"
exit 0
