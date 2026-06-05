---
score_cap:
  - criterion: "micro-phase-wave-1 never reappears in state.json carryoverTodos (shipped cycle 217, a354d85)"
    max_if_missing: 4
    evidence: "python3 -c \"import json,sys; s=json.load(open('.evolve/state.json')); sys.exit(1 if any(t.get('id')=='micro-phase-wave-1' for t in s.get('carryoverTodos',[])) else 0)\""
  - criterion: "completed carryoverTodos are retired with a positive closure record, not silently deleted"
    max_if_missing: 3
    evidence: "python3 -c \"import json,sys; s=json.load(open('.evolve/state.json')); sys.exit(0 if any(t.get('id')=='micro-phase-wave-1' and t.get('decision')=='completed' for t in s.get('evaluatedTasks',[])) else 1)\""
---

# Eval: Retire the shipped micro-phase-wave-1 carryoverTodo

> Pins the closure of the wave-1 micro-phase carryoverTodo. Wave 1 fully
> shipped in cycle 217 (commit a354d85: all 7 phases pass `evolve phases
> validate`), but the todo was never removed from
> `state.json:carryoverTodos[]`, so cycle 218 re-spawned the identical goal
> hash and burned a full scout/triage pass re-discovering "already done".
> The closeout removes the entry AND records it in `evaluatedTasks[]` with
> `decision=completed` + `completed_cycle=217` — closure must be a positive
> record, not a silent delete. Source incident: cycle 218 (2026-06-05),
> scout reflection: "wave-1 was shipped but the carryoverTodo was never
> retired, causing the same goal to re-fire." Caps are modest because
> state.json is mutable runtime state subject to future compaction; the
> carryover-absence criterion is the load-bearing one.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| zombie-todo re-fire | wave-1 absent from carryoverTodos | 4/10 | python3 absence assert on `.evolve/state.json` |
| silent-delete closure | wave-1 closure recorded in evaluatedTasks | 3/10 | python3 evaluatedTasks assert on `.evolve/state.json` |
