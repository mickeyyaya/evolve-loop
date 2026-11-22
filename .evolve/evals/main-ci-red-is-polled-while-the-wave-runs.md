---
score_cap:
  - criterion: "The loop tick opens the main-red stop and emits GATE_REPO_MAIN_CI_RED when origin/main CI is red"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestLoopTick_MainCIRedOpensMainRedStopAndEmitsIncident$' ./cmd/evolve"
  - criterion: "A CI-only red opens the stop with no lane reproduction"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestMainRedStop_CIOnlyRedNeedsNoLaneReproduction$' ./internal/core"
  - criterion: "Ship refuses a landing with a typed cause while the CI stop is open"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestShip_RefusesLandingWhileOriginMainCIRed$' ./internal/phases/ship"
  - criterion: "A green main closes the stop and landings resume"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestLoopTick_MainCIGreenClosesTheStopAndConsumesTheFixItem)$' ./cmd/evolve && go test -count=1 -run '^TestShip_LandingsResumeWhenAGreenMainClosesTheCIStop$' ./internal/phases/ship"
  - criterion: "At enforce the lane that carries the CI stop fix item lands, so the loop heals a red main CI through evolve ship"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestShip_AdmitsTheLaneThatCarriesTheCIStopFixItemAtEnforce|TestShip_AdmitsTheCIFixLaneThatTheTriageDecisionCommits)$' ./internal/phases/ship"
  - criterion: "Another lane, a lane-key stop and a mixed stop stay refused whatever the lane carries"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestShip_RefusesAnotherLaneWhileTheCIStopHoldsEvenWhenItHasAScope|TestShip_CarryingTheFixItemOfALaneKeyStopStillNeedsTheKeyProof|TestShip_CarryingTheFixItemOfAMixedStopStillNeedsTheKeyProof)$' ./internal/phases/ship"
  - criterion: "An unknown CI state (query error, no completed check-run) opens no stop"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestLoopTick_MainCIUnknownOpensNoStopAndEmitsNoIncident$' ./cmd/evolve"
---

# Eval: main CI red is polled while the wave runs

> Pins the loop-tick poll of origin/main CI, the CI-only main-red stop, the ship refusal and the release on green.
> Source incident: 2026-11-20, main CI red at 02:11Z after cycle 2554; three more landings (2555, 2550, 2549) reached the red main because the only CI poll ran at the next wave start (inbox item main-ci-red-is-polled-while-the-wave-runs, cycle 2587). The audit finding M1 of cycle 2587 added the fix-lane admission: at enforce a CI-only stop refused the fix lane too, so the loop could not heal a red main CI by itself.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tick-poll | red CI opens the stop and emits the incident | 4/10 | `go test -run TestLoopTick_MainCIRedOpensMainRedStopAndEmitsIncident ./cmd/evolve` |
| ci-only-open | the stop needs no lane reproduction | 4/10 | `go test -run TestMainRedStop_CIOnlyRedNeedsNoLaneReproduction ./internal/core` |
| ship-refusal | ship refuses with a typed cause | 4/10 | `go test -run TestShip_RefusesLandingWhileOriginMainCIRed ./internal/phases/ship` |
| green-resume | green closes the stop, landings resume | 5/10 | `go test -run TestShip_LandingsResumeWhenAGreenMainClosesTheCIStop ./internal/phases/ship` |
| ci-fix-lane | the lane that carries the CI stop fix item lands at enforce | 5/10 | `go test -run TestShip_AdmitsTheLaneThatCarriesTheCIStopFixItemAtEnforce ./internal/phases/ship` |
| ci-fix-lane-negatives | no other lane, lane-key stop or mixed stop is admitted by carrying an item | 5/10 | `go test -run TestShip_RefusesAnotherLaneWhileTheCIStopHoldsEvenWhenItHasAScope ./internal/phases/ship` |
| unknown-ci | CI query failure opens no stop | 6/10 | `go test -run TestLoopTick_MainCIUnknownOpensNoStopAndEmitsNoIncident ./cmd/evolve` |
