#!/usr/bin/env bash
# ACS cycle-218 / task wave1-carryover-closeout AC2 — the wave-1 closure is
# RECORDED, not just deleted: state.json:evaluatedTasks[] gains an entry
# {"id":"micro-phase-wave-1","decision":"completed","completed_cycle":217,...}.
#
# This is the anti-no-op half of the closeout: predicate 005 (absence) would
# pass on an empty state.json; this one requires the positive closure record.
# Behavioral-on-side-effect via python3 JSON parse (see 005 header for the
# git-tracking note).
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
STATE="$TOP/.evolve/state.json"
[ -f "$STATE" ] || { echo "RED: $STATE missing" >&2; exit 1; }

if python3 - "$STATE" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
entries = [t for t in s.get("evaluatedTasks", [])
           if t.get("id") == "micro-phase-wave-1"]
ok = bool(entries) and entries[0].get("decision") == "completed" \
     and entries[0].get("completed_cycle") == 217
sys.exit(0 if ok else 1)
PY
then
  echo "GREEN: micro-phase-wave-1 closure recorded in evaluatedTasks (decision=completed, cycle 217)" >&2
  exit 0
fi
echo "RED: evaluatedTasks lacks micro-phase-wave-1 with decision=completed + completed_cycle=217" >&2
exit 1
