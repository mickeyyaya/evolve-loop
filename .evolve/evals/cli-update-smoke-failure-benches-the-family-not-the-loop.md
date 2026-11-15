---
score_cap:
  - criterion: "A family that fails its post-update smoke is benched and the boot and the wave boundary go on while a routed family can run"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestBootCLIUpdate_AFamilyThatFailsItsSmokeIsBenchedAndTheWaveStarts|TestBootCLIUpdate_TheRoutesSkipAFamilyBenchedBySmoke|TestUpdateCLIsAtBoundary_AFamilyThatFailsItsSmokeIsBenchedAndTheNextWaveRuns|TestBootGateHalts_ABenchedSmokeFailureLetsTheReadinessGateRun)$' ./cmd/evolve"
  - criterion: "When no route has a family left to run, or no routing table proves one, the boot and the boundary halt as before"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestBootCLIUpdate_AFamilyThatNoRouteCanDoWithoutHaltsTheLoop|TestBootCLIUpdate_ASmokeFailureWithNoRoutingTableStillHaltsTheLoop|TestUpdateCLIsAtBoundary_EveryRoutedFamilyFailingItsSmokeHaltsTheNextWave|TestRunLoop_ASmokeFailureAtBootHaltsBeforeAnyCycleNamingTheFamilyAndVersion|TestPrepareIteration_ASmokeFailureAtAWaveBoundaryHaltsTheNextWave)$' ./cmd/evolve"
---

# Eval: A smoke failure benches the CLI family, not the loop

> Pins the boot and boundary behavior of the CLI updater. A family whose
> post-update smoke fails goes on the clihealth bench with an expiry. The loop
> halts only when no route has a family left to run. Source incident: cycle
> 2406, the agy 1.5.3 to 1.5.9 update (rc=87) that stopped the whole loop while
> claude was healthy.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| bench-and-continue | agy fails, claude passes: agy benched, one INCIDENT, wave starts, routes skip agy | 5/10 | `go test -run TestBootCLIUpdate_AFamilyThatFailsItsSmokeIsBenchedAndTheWaveStarts ./cmd/evolve` |
| halt-as-today | every routed family fails, a pinned role loses its only family, or no table exists: halt | 6/10 | `go test -run TestBootCLIUpdate_AFamilyThatNoRouteCanDoWithoutHaltsTheLoop ./cmd/evolve` |
