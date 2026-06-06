#!/usr/bin/env bash
# ACS cycle-218 / task wave1-carryover-closeout AC3 — state.json remains valid
# JSON after the closeout edit. Pre-existing GREEN at RED baseline (the file
# is valid before the edit) — this predicate is a corruption GUARD: triage
# flagged that state.json appears in 22 regression predicates, so a botched
# hand-edit here breaks the whole suite.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
STATE="$TOP/.evolve/state.json"
[ -f "$STATE" ] || { echo "RED: $STATE missing" >&2; exit 1; }

if python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$STATE" 2>/dev/null; then
  echo "GREEN: state.json parses as valid JSON" >&2
  exit 0
fi
echo "RED: state.json is corrupt — closeout edit broke it" >&2
exit 1
