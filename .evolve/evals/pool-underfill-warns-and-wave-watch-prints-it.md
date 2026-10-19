---
score_cap:
  - criterion: "An idle transition with live below fleet.min_lanes emits LOOP_ORCH_POOL_UNDERFILLED at WARN"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestUnderfill_AnIdleTransitionBelowMinLanesWarnsOnceWithTheFacts$' ./internal/orchestrator"
  - criterion: "No UNDERFILLED WARN at or above min_lanes or while the operator brake is engaged"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestUnderfill_(NoWarnWhileTheLiveCountHoldsMinLanes|NoWarnWhileTheOperatorBrakeIsEngaged)$' ./internal/orchestrator"
  - criterion: "The loop hands fleet.min_lanes to the pool it builds"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestOrchestratorPool_WiresTheEnforceKeysAndTheFileRuleWithoutAGraph$' ./cmd/evolve"
  - criterion: "evolve wave watch prints the pool transitions and every loop.wave WARN"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWaveWatch_PrintsThePoolTransitionsAndEveryLoopWaveWarn$' ./cmd/evolve"
---

# Eval: An idle pool below fleet.min_lanes raises a WARN, and wave watch prints the pool events

> Pins the starvation signal added after the wave 130 investigation of 2026-10-19. The pool published
> LOOP_ORCH_POOL_IDLE seven times at live 1-3 of 4 over three hours, and `evolve wave watch` printed only
> per-cycle phase lines, so a starved pool looked like a busy one. The pool now warns on each idle transition
> below min_lanes, and the watch prints every pool transition and every loop.wave WARN.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| warn-below-min | idle below min_lanes gives one WARN with live, target, min_lanes, ready | 6/10 | `go test -run TestUnderfill_AnIdleTransitionBelowMinLanesWarnsOnceWithTheFacts ./internal/orchestrator` |
| no-false-warn | no WARN at min_lanes or under the brake | 7/10 | `go test -run TestUnderfill_NoWarn ./internal/orchestrator` |
| caller-wired | the loop sets Pool.MinLanes from fleet.min_lanes | 7/10 | `go test -run TestOrchestratorPool_WiresTheEnforceKeys ./cmd/evolve` |
| watch-prints | wave watch prints pool lines and WARNs | 6/10 | `go test -run TestWaveWatch_PrintsThePoolTransitionsAndEveryLoopWaveWarn ./cmd/evolve` |
