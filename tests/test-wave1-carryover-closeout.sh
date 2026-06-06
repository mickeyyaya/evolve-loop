#!/usr/bin/env bash
# tests/test-wave1-carryover-closeout.sh — cycle 218, task wave1-carryover-closeout
#
# RED contract for Builder: retire the shipped micro-phase-wave-1 carryoverTodo
# from .evolve/state.json so cycle 219 stops re-spawning the same goal hash.
#   - REMOVE micro-phase-wave-1 from carryoverTodos[]
#   - APPEND {"id":"micro-phase-wave-1","decision":"completed","completed_cycle":217,...}
#     to evaluatedTasks[] (create the key if absent)
#   - DO NOT touch micro-phase-wave-2 / micro-phase-wave-3
#
# NOTE: .evolve/state.json in the worktree is a SYMLINK to the main tree's
# runtime state — it is deliberately NOT git-tracked, so the file-existence
# dual-check (git ls-files) does not apply here. python3 JSON assertions on
# the resulting state ARE the observable side effect of this task.
set -uo pipefail

TOP=$(git rev-parse --show-toplevel) || { echo "FAIL: not a git repo"; exit 1; }
STATE="$TOP/.evolve/state.json"
PASS=0; FAIL=0

ok()  { echo "PASS: $1"; PASS=$((PASS+1)); }
bad() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

py() { python3 -c "$1" "$STATE" >/dev/null 2>&1; }

# --- 1. state.json is valid JSON (guard: the edit must not corrupt it) -------
if py 'import json,sys; json.load(open(sys.argv[1]))'; then
  ok "state.json parses as valid JSON"
else
  bad "state.json is missing or invalid JSON"
fi

# --- 2. NEGATIVE: micro-phase-wave-1 must be ABSENT from carryoverTodos ------
if py 'import json,sys
s=json.load(open(sys.argv[1]))
assert not any(t.get("id")=="micro-phase-wave-1" for t in s.get("carryoverTodos",[]))'; then
  ok "micro-phase-wave-1 absent from carryoverTodos"
else
  bad "micro-phase-wave-1 still present in carryoverTodos"
fi

# --- 3. micro-phase-wave-1 recorded in evaluatedTasks with decision=completed
if py 'import json,sys
s=json.load(open(sys.argv[1]))
assert any(t.get("id")=="micro-phase-wave-1" and t.get("decision")=="completed"
           for t in s.get("evaluatedTasks",[]))'; then
  ok "micro-phase-wave-1 in evaluatedTasks with decision=completed"
else
  bad "micro-phase-wave-1 missing from evaluatedTasks (or decision != completed)"
fi

# --- 4. closure entry cites the delivering cycle (217) -----------------------
if py 'import json,sys
s=json.load(open(sys.argv[1]))
e=[t for t in s.get("evaluatedTasks",[]) if t.get("id")=="micro-phase-wave-1"]
assert e and e[0].get("completed_cycle")==217'; then
  ok "evaluatedTasks entry records completed_cycle=217"
else
  bad "evaluatedTasks entry missing completed_cycle=217"
fi

# --- 5-6. NEGATIVE: waves 2 and 3 must NOT be removed (edit must not overshoot)
for wave in micro-phase-wave-2 micro-phase-wave-3; do
  if py "import json,sys
s=json.load(open(sys.argv[1]))
assert any(t.get('id')=='$wave' for t in s.get('carryoverTodos',[]))"; then
    ok "$wave still present in carryoverTodos"
  else
    bad "$wave was removed from carryoverTodos — closeout overshot"
  fi
done

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
