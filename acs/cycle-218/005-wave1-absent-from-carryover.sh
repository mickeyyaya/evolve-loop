#!/usr/bin/env bash
# ACS cycle-218 / task wave1-carryover-closeout AC1 — micro-phase-wave-1 is
# retired from state.json:carryoverTodos[] (it shipped in cycle 217, a354d85;
# the stale entry caused cycle 218 to re-spawn the same goal hash).
#
# Behavioral-on-side-effect: the deliverable of this task IS the state.json
# mutation; python3 parses the live file and asserts on its structure (no
# source grepping). state.json is runtime state — deliberately NOT
# git-tracked (worktree symlinks it to the main tree), so the file-tracking
# dual-check does not apply.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
STATE="$TOP/.evolve/state.json"
[ -f "$STATE" ] || { echo "RED: $STATE missing" >&2; exit 1; }

if python3 - "$STATE" <<'PY'
import json, sys
s = json.load(open(sys.argv[1]))
sys.exit(1 if any(t.get("id") == "micro-phase-wave-1"
                  for t in s.get("carryoverTodos", [])) else 0)
PY
then
  echo "GREEN: micro-phase-wave-1 absent from carryoverTodos" >&2
  exit 0
fi
echo "RED: micro-phase-wave-1 still present in carryoverTodos" >&2
exit 1
