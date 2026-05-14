#!/usr/bin/env bash
# AC2: subagent-run.sh has turn-overrun detection that appends abnormal event
# predicate: T2 — turn-overrun observability wired into usage sidecar processing
# metadata: cycle=47 task=T2 ac=AC2 risk=low
set -euo pipefail
REPO_ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel 2>/dev/null || echo ".")"
TARGET="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
[ -f "$TARGET" ] || { echo "FAIL: $TARGET not found"; exit 1; }
grep -q "turn-overrun" "$TARGET" || { echo "FAIL: turn-overrun event not in subagent-run.sh"; exit 1; }
grep -q "_actual_turns" "$TARGET" || { echo "FAIL: _actual_turns comparison not in subagent-run.sh"; exit 1; }
grep -q "_max_turns_profile" "$TARGET" || { echo "FAIL: _max_turns_profile comparison not in subagent-run.sh"; exit 1; }
echo "PASS: subagent-run.sh has turn-overrun detection (T2)"
