---
score_cap:
  - criterion: "A braked pool that also reached run_max_lanes reports operator_stop on its idle event and emits no underfill"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPool_ABrakedPoolPastItsRunCapNamesTheOperatorStop$' ./internal/orchestrator/"
  - criterion: "The loop pool run (orchestratedRun) reports operator_stop when the loop-stop brake meets the run cap"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestOrchestratedRun_AWaveStopThatMeetsTheRunCapNamesTheOperatorStop$' ./cmd/evolve/"
  - criterion: "An unbraked pool past its run cap still reports run_cap"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPool_UnderfilledEventNamesTheRunCapCause$' ./internal/orchestrator/"
  - criterion: "The ACS predicates of cycle 2432 pass"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 ./acs/cycle2432"
---

# Eval: A wave stop that drains a pool reports operator_stop

> Pins the idle cause order of the orchestrator pool: the operator stop (loop-stop brake, `evolve wave stop`) wins over the run cap (`fleet.orchestrator.run_max_lanes`). Source incident: wave 171, 2026-11-14: PQ8 read operator-stop drain time as run_cap. The pool code already holds the order, so this eval keeps both the pool level and the loop level from regressing, and pins the package note.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| braked-and-capped | idle cause is operator_stop, no underfill | 6/10 | `go test -run TestPool_ABrakedPoolPastItsRunCapNamesTheOperatorStop` |
| loop wiring | production caller reports operator_stop | 7/10 | `go test -run TestOrchestratedRun_AWaveStopThatMeetsTheRunCapNamesTheOperatorStop` |
| unbraked-and-capped | cause stays run_cap | 6/10 | `go test -run TestPool_UnderfilledEventNamesTheRunCapCause` |
| predicates | cycle 2432 ACS | 7/10 | `go test -tags acs ./acs/cycle2432` |
