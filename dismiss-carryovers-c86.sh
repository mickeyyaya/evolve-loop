#!/usr/bin/env bash
# dismiss-carryovers-c86.sh — Builder cycle-86 dismissal script.
# Removes abnormal-ship-refused-c86 and abnormal-turn-overrun-c86 from
# state.json:carryoverTodos[] and archives them with rationale.
# Uses atomic tmp+mv; no jq -i; bash 3.2 compatible.
set -uo pipefail

_self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Resolve project root from worktree via resolve-roots.sh
if [ -f "$_self_dir/scripts/lifecycle/resolve-roots.sh" ]; then
    . "$_self_dir/scripts/lifecycle/resolve-roots.sh" 2>/dev/null || true
fi
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git -C "$_self_dir" rev-parse --git-common-dir 2>/dev/null | sed 's|/.git$||' || pwd)}"
# Fallback: climb from common-dir
if [ -z "${EVOLVE_PROJECT_ROOT:-}" ]; then
    _common="$(git -C "$_self_dir" rev-parse --git-common-dir 2>/dev/null || true)"
    if [ -n "$_common" ]; then
        PROJECT_ROOT="$(cd "$_self_dir" && cd "$_common/.." && pwd)"
    fi
fi

STATE="$PROJECT_ROOT/.evolve/state.json"
ARCHIVE_DIR="$PROJECT_ROOT/.evolve/archive/lessons"
ARCHIVE_FILE="$ARCHIVE_DIR/carryover-todos-archive.jsonl"

echo "[dismiss-c86] PROJECT_ROOT=$PROJECT_ROOT"
echo "[dismiss-c86] STATE=$STATE"

[ -f "$STATE" ] || { echo "[dismiss-c86] ERROR: state.json not found at $STATE"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "[dismiss-c86] ERROR: jq required"; exit 1; }

mkdir -p "$ARCHIVE_DIR"
TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Archive both dismissed todos with rationale before removal
jq -c --arg ts "$TS" --argjson cyc 86 '
  .carryoverTodos[] |
  select(.id == "abnormal-ship-refused-c86" or .id == "abnormal-turn-overrun-c86") |
  if .id == "abnormal-ship-refused-c86" then
    . + {archived_at: $ts, archived_at_cycle: $cyc, archive_reason: "completed-pass",
         dismissal_rationale: "No ship-refused event found in any abnormal-events.jsonl. Inbox-projector false-positive from v10.x carryover-projection pass. Scout-confirmed; zero evidence trail."}
  else
    . + {archived_at: $ts, archived_at_cycle: $cyc, archive_reason: "completed-pass",
         dismissal_rationale: "Real WARN-level turn-overrun from cycle-86 intent phase (16/10 turns). Pipeline completed normally. IMKI investigation-class goal exceeds 10-turn ceiling — design constraint, not a code defect. Scout-confirmed, Triage-gated."}
  end
' "$STATE" >> "$ARCHIVE_FILE"

echo "[dismiss-c86] archived both entries to $ARCHIVE_FILE"

# Remove both IDs from active carryoverTodos[] atomically
TMP="${STATE}.tmp.$$"
jq '
  .carryoverTodos = [
    .carryoverTodos[] |
    select(.id != "abnormal-ship-refused-c86" and .id != "abnormal-turn-overrun-c86")
  ]
' "$STATE" > "$TMP" && mv "$TMP" "$STATE"

echo "[dismiss-c86] removed both carryovers from state.json:carryoverTodos[]"

# Verify removal
remaining=$(jq '[.carryoverTodos[] | select(.id == "abnormal-ship-refused-c86" or .id == "abnormal-turn-overrun-c86")] | length' "$STATE")
echo "[dismiss-c86] remaining matches: $remaining"
if [ "$remaining" -ne 0 ]; then
    echo "[dismiss-c86] ERROR: removal failed ($remaining still present)"
    exit 1
fi

echo "[dismiss-c86] DONE: both carryovers dismissed successfully"
