---
score_cap:
  - criterion: "carryover list --json prints every todo with age and cycles_unpicked"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestCarryoverList$' ./cmd/evolve"
  - criterion: "listed ids are accepted verbatim by apply-decisions"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestCarryoverList_AcceptanceWithApplyDecisions$' ./cmd/evolve"
  - criterion: "unreadable or malformed state.json exits 2"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestCarryoverList_UnreadableStateExits2$' ./cmd/evolve"
  - criterion: "empty carryover lists as [] with exit 0"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestCarryoverList_EmptyCarryover$' ./cmd/evolve"
  - criterion: "runtime-reference.md documents evolve carryover list"
    max_if_missing: 7
    evidence: "grep -q 'evolve carryover list' docs/operations/runtime-reference.md"
---

# Eval: cli-carryover-list

> Pins the read-only `evolve carryover list [--json] [--sort age|unpicked] [--limit N] [--state PATH]` subcommand. Source: inbox item cli-carryover-list (2026-09-30): carryoverTodos could only be reviewed with jq on state.json. Age = lastCycleNumber - first_seen_cycle.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| json-rows | age and cycles_unpicked per todo | 4/10 | `go test -run ^TestCarryoverList$` |
| id-compat | ids accepted by apply-decisions | 5/10 | `go test -run ^TestCarryoverList_AcceptanceWithApplyDecisions$` |
| exit-2 | unreadable state exits 2 | 6/10 | `go test -run ^TestCarryoverList_UnreadableStateExits2$` |
| empty | empty queue is [] exit 0 | 7/10 | `go test -run ^TestCarryoverList_EmptyCarryover$` |
| docs | documented in runtime-reference.md | 7/10 | `grep -q 'evolve carryover list' docs/operations/runtime-reference.md` |
