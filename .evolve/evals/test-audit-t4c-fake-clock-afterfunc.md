---
score_cap:
  - criterion: "the contract table passes on the fake clock and on SystemClock, lock case included"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestClockContract$' ./internal/orchestrator/"
  - criterion: "the lock case can tell an inline clock from time.AfterFunc"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine' ./internal/orchestrator/"
---

# Eval: orchestrator fake clock runs AfterFunc callbacks off the caller goroutine

> Pins the clock contract of `go/internal/orchestrator`: the fake clock and `SystemClock`
> behave the same when a callback takes a lock the caller holds. The old fake ran the
> callback inline in `Advance`, so such a callback deadlocked the fake and not the real clock.
> Source: inbox item test-audit-t4c-fake-clock-afterfunc (cycle 2511, T4).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| contract-table | both clocks pass every case | 6/10 | `go test -run '^TestClockContract$' ./internal/orchestrator/` |
| non-vacuous-lock-case | the lock case fails on an inline clock | 7/10 | `go test -run TestClockContract_CatchesAClockThatRunsCallbacksOnTheCallersGoroutine ./internal/orchestrator/` |
