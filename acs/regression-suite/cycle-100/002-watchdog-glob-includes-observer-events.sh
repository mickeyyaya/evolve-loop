#!/usr/bin/env bash
# AC-ID: cycle-100-002-watchdog-glob-includes-observer-events
# AC-source: cycle-100/intent.md AC "phase-watchdog.sh glob loop includes *-observer-events.ndjson."
#
# Behavioral predicate: phase-watchdog.sh's activity-scan glob loop MUST
# include `*-observer-events.ndjson`. This is the defensive glob added to
# the opt-out path so operators who set EVOLVE_OBSERVER_ENFORCE=0 still
# get an activity signal from the observer's event file.
#
# Strategy:
#   1. Source-level dual-check: confirm the glob string is present.
#   2. Behavioral subprocess check: spawn phase-watchdog.sh against a
#      synthetic workspace that contains ONLY a recent
#      `xyz-observer-events.ndjson` (no .log/.md/.json files). With a
#      short STALL_S, the watchdog must NOT fire stall-detected within
#      the observation window — i.e. it must recognize the observer
#      file as activity.
#
# Bash 3.2 compatible. No GNU date/sed flags.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="scripts/dispatch/phase-watchdog.sh"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET missing on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1; then
  echo "RED: $TARGET exists but is not git-tracked" >&2
  exit 1
fi

# (1) Source-level: locate the glob loop and ensure the observer-events
# pattern is one of its entries.
if ! grep -q 'observer-events\.ndjson' "$TARGET"; then
  echo "RED: $TARGET does not mention 'observer-events.ndjson' anywhere" >&2
  exit 1
fi

# Tighter check: the pattern must appear *inside* the glob loop block.
# The block begins with `for glob_pat in` and ends with `; do`.
# Extract the block via awk and grep within it.
in_block=$(awk '
  /for glob_pat in/ { in_block=1 }
  in_block { print }
  in_block && /; do$/ { in_block=0 }
' "$TARGET")
if [ -z "$in_block" ]; then
  echo "RED: could not locate 'for glob_pat in ...' block in $TARGET" >&2
  exit 1
fi
if ! printf '%s\n' "$in_block" | grep -q 'observer-events\.ndjson'; then
  echo "RED: 'observer-events.ndjson' not in the glob_pat loop block of $TARGET" >&2
  printf '%s\n' "$in_block" >&2
  exit 1
fi

# (2) Behavioral test — run phase-watchdog briefly against a synthetic
# workspace.
TMP_DIR="$(mktemp -d -t acs-cycle-100-002.XXXXXX)" || {
  echo "RED: mktemp -d failed" >&2
  exit 1
}
WS="$TMP_DIR/workspace"
mkdir -p "$WS"
trap '
  if [ -n "${WD_PID:-}" ] && kill -0 "$WD_PID" 2>/dev/null; then
    kill -TERM "$WD_PID" 2>/dev/null || true
    sleep 1
    kill -KILL "$WD_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP_DIR"
' EXIT

# Workspace contains ONLY a recent observer-events file. If the glob
# loop does not include the new pattern, mtime probing will see no
# activity and the watchdog WILL emit stall-detected when its threshold
# elapses.
NDJSON="$WS/phase-builder-observer-events.ndjson"
printf '{"event_type":"heartbeat","timestamp":"%s"}\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" >"$NDJSON"

# Cycle-state stub (avoid watchdog erroring on missing path).
CS_PATH="$TMP_DIR/cycle-state.json"
printf '{"cycle_number":100,"orchestrator_phase_log":[]}\n' >"$CS_PATH"

# Run watchdog with very short stall threshold. PGID = our own (it will
# never receive a real TERM signal because we kill the watchdog before
# it fires; we only care whether stall-detected is logged).
RUN_PGID=$(ps -o pgid= -p $$ 2>/dev/null | tr -d ' ')
LOG_FILE="$TMP_DIR/watchdog.log"

# 5-second threshold; sleep 8s; expectation: if observer-events glob is
# scanned, the freshly-touched ndjson will keep best_mtime current and
# no stall fires. If the glob is missing, best_mtime never advances
# beyond epoch-zero and stall fires.
EVOLVE_INACTIVITY_THRESHOLD_S=5 \
EVOLVE_WATCHDOG_POLL_S=1 \
  bash "$REPO_ROOT/$TARGET" "$WS" "$RUN_PGID" 100 "$CS_PATH" >"$LOG_FILE" 2>&1 &
WD_PID=$!

# Keep the ndjson "fresh" — touch every second.
end_ts=$(( $(date +%s) + 8 ))
while [ "$(date +%s)" -lt "$end_ts" ]; do
  touch "$NDJSON"
  if ! kill -0 "$WD_PID" 2>/dev/null; then
    break
  fi
  sleep 1
done

# Terminate watchdog cleanly.
if kill -0 "$WD_PID" 2>/dev/null; then
  kill -TERM "$WD_PID" 2>/dev/null || true
  sleep 1
  kill -KILL "$WD_PID" 2>/dev/null || true
fi

# Verdict: stall-detected MUST NOT appear in the log.
if grep -q 'stall-detected' "$LOG_FILE"; then
  echo "RED: phase-watchdog emitted stall-detected against a workspace whose only fresh file was observer-events.ndjson" >&2
  echo "--- watchdog log ---" >&2
  cat "$LOG_FILE" >&2 || true
  echo "--- end log ---" >&2
  exit 1
fi

echo "GREEN: $TARGET glob loop includes 'observer-events.ndjson'; behavioral run against synthetic workspace did NOT emit stall-detected"
exit 0
