---
score_cap:
  - criterion: "A Stop before the delay still returns true when the caller is descheduled past the delay"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestClockContract_StopBeforeTheDelayHoldsWhenTheCallerIsDescheduled$' ./internal/orchestrator/"
  - criterion: "The stop-before-delay case still fails on a clock whose Stop does not cancel the run, so the delay was not widened past proof"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestClockContract_StopBeforeTheDelayCatchesAClockWhoseStopDoesNotCancelTheRun$' ./internal/orchestrator/"
  - criterion: "The whole clock contract passes for the fake and the system clock under the race detector"
    max_if_missing: 6
    evidence: "cd go && go test -race -count=1 -run '^TestClockContract$' ./internal/orchestrator/"
---

# Eval: the system-clock stop-before-delay case holds under load

> Pins the fix for the flaky system-clock case of the clock contract. The case armed its
> to-be-stopped timer with the 1ms system delay, so a descheduled caller let the timer fire
> before Stop and Stop returned false. A one-hour delay on that timer removes the race but also
> removes the proof: a Stop that does not cancel would never be seen to run. The mutant guard
> keeps the delay inside the proof window. Source incident: cycle 2673.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| descheduled-caller | Stop stays true when the caller stalls past the delay | 5/10 | `go test -run TestClockContract_StopBeforeTheDelayHoldsWhenTheCallerIsDescheduled` |
| no-over-widening | a Stop that does not cancel is still caught | 6/10 | `go test -run TestClockContract_StopBeforeTheDelayCatchesAClockWhoseStopDoesNotCancelTheRun` |
| contract-unchanged | the five contract cases pass on both clocks | 6/10 | `go test -race -run TestClockContract` |
