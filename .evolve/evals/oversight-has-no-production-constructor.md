---
score_cap:
  - criterion: "orchestratedRun constructs and runs orchestrator.Oversight and stops it with the pool"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestOrchestratedRun_ConstructsAndRunsOversightAndStopsItWithThePool$'"
  - criterion: "a clean landing moves the adaptive window at enforce through the oversight"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestOrchestratedRun_ACleanLandingMovesTheAdaptiveWindowAtEnforce$'"
  - criterion: "a landing at shadow is recorded unapplied and moves nothing"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestOrchestratedRun_ALandingAtShadowIsRecordedAndMovesNothing$'"
  - criterion: "an event source that cannot open leaves the pool running with a WARN"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestOrchestratedRun_AnEventSourceThatCannotOpenLeavesThePoolRunningWithAWarning$'"
  - criterion: "the oversight halt reaches the pool intake when cell admission loads"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestOrchestratedRun_TheOversightHaltReachesThePoolIntakeWhenCellAdmissionLoads$'"
---

# Eval: oversight-has-no-production-constructor

> Pins the loop wiring of `orchestrator.Oversight`. Before cycle 2427 the type had no production
> constructor, so no `ship.landed` event ever called `Window.CleanLanding` and the adaptive window
> stayed at `window.start`. The tests drive the production seam `orchestratedRun` with a scripted
> event source and read the signals it emits; a test that calls `Window.CleanLanding` directly
> cannot pass them.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wiring | the loop constructs, runs and stops the oversight | 4/10 | `go test -run TestOrchestratedRun_ConstructsAndRunsOversightAndStopsItWithThePool ./cmd/evolve` |
| journey | a clean landing moves the window 2 to 3 at enforce | 4/10 | `go test -run TestOrchestratedRun_ACleanLandingMovesTheAdaptiveWindowAtEnforce ./cmd/evolve` |
| negative | shadow records `applied=false` and moves nothing | 6/10 | `go test -run TestOrchestratedRun_ALandingAtShadowIsRecordedAndMovesNothing ./cmd/evolve` |
| edge | a refused event source degrades to a WARN | 6/10 | `go test -run TestOrchestratedRun_AnEventSourceThatCannotOpenLeavesThePoolRunningWithAWarning ./cmd/evolve` |
| shared intake | the halt event stops the pool through its own intake | 6/10 | `go test -run TestOrchestratedRun_TheOversightHaltReachesThePoolIntakeWhenCellAdmissionLoads ./cmd/evolve` |
