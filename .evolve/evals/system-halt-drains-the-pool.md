---
score_cap:
  - criterion: "A lane that exits with the system-failure halt code stops the pool backfill at once"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2594_001|TestC2594_004' ./acs/cycle2594"
  - criterion: "The idle cause after a system-failure halt is system_halt, a member of the closed IdleCauses set, and an operator brake stays operator_stop"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2594_002|TestC2594_003' ./acs/cycle2594"
  - criterion: "The wave ends when the last running lane seals after a halt, and LOOP_ORCH_HALT_DRAINING is raised as an INCIDENT with the cause"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2594_005|TestC2594_006' ./acs/cycle2594"
  - criterion: "A pool without a halt still backfills and raises no halt-draining incident"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2594_007' ./acs/cycle2594"
  - criterion: "The production caller orchestratedRun and the package suite pin the same behavior"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2594_008|TestC2594_009' ./acs/cycle2594"
  - criterion: "The LOOP_ORCH_HALT_DRAINING incident carries every field its registry doc names"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2594_010|TestC2594_011' ./acs/cycle2594"
---

# Eval: after a system-failure halt the pool drains instead of backfilling

> Pins the pool behavior after a lane exits with the ADR-0072 system-failure halt code (rc 4).
> Source incident: wave 182, where the pool backfilled lanes after LOOP_SYSTEM_FAILURE_HALT
> and then idled 3h38m with the cause operator_stop. Cycle 2594 closes it.

## Graders
- [code] `cd go && go test -count=1 ./internal/orchestrator -run 'TestPool_NoBackfillAfterSystemFailureHalt|TestIdleCause_SystemHaltIsNotOperatorStop'` exits 0 (both tests red before the change).
- [code] `cd go && go test -count=1 ./internal/orchestrator ./cmd/evolve` exits 0, no regression.
- [code] Negative: a pool with no halt still backfills (existing pool tests pass); idle cause for a plain Stopped transition stays operator_stop.
- [code] Edge: halt raised with zero running lanes ends the wave at once; halt with N running lanes ends it after the last seals.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| no-backfill | Halt stops the backfill, also with one lane | 4/10 | `TestC2594_001`, `TestC2594_004` |
| idle-cause | system_halt is its own closed-set cause | 5/10 | `TestC2594_002`, `TestC2594_003` |
| drain-end | The wave ends at the last seal, with the INCIDENT | 5/10 | `TestC2594_005`, `TestC2594_006` |
| no-halt-negative | No halt means backfill and no incident | 6/10 | `TestC2594_007` |
| caller-proof | The loop caller and the package suite pin it | 6/10 | `TestC2594_008`, `TestC2594_009` |
| halt-incident-fields | The incident carries the fields its registry doc names, once per halt | 7/10 | `TestC2594_010`, `TestC2594_011` |
