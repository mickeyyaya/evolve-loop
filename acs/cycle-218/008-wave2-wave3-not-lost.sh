#!/usr/bin/env bash
# ACS cycle-218 / task wave1-carryover-closeout AC4 — NEGATIVE/overshoot guard:
# retiring wave-1 must NOT drop micro-phase-wave-2 or micro-phase-wave-3.
# Pre-existing GREEN at RED baseline (both are present before the edit) —
# this pins that the closeout edit is surgical.
#
# Forward-compatible form (cycle-89/100 staleness lesson — the very bug this
# cycle fixes): each wave passes if it is EITHER still queued in
# carryoverTodos OR legitimately closed in evaluatedTasks with
# decision=completed. So this predicate stays green when waves 2/3 ship in
# future cycles, but goes RED if either entry is silently LOST.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
STATE="$TOP/.evolve/state.json"
[ -f "$STATE" ] || { echo "RED: $STATE missing" >&2; exit 1; }

if python3 - "$STATE" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
carry = {t.get("id") for t in s.get("carryoverTodos", [])}
done = {t.get("id") for t in s.get("evaluatedTasks", [])
        if t.get("decision") == "completed"}
missing = [w for w in ("micro-phase-wave-2", "micro-phase-wave-3")
           if w not in carry and w not in done]
if missing:
    print("LOST: " + ", ".join(missing), file=sys.stderr)
    sys.exit(1)
sys.exit(0)
PY
then
  echo "GREEN: waves 2 and 3 each still queued (or legitimately completed) — closeout did not overshoot" >&2
  exit 0
fi
echo "RED: a wave-2/3 entry was silently lost from state.json" >&2
exit 1
