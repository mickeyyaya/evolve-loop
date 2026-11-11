---
score_cap:
  - criterion: "A sealed cycle with a dirty lane and an orphan process is cleaned at once, through the gc sweep"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestSweepSealedCycle_ASealedDirtyLaneWithAnOrphanIsCleanedAtOnce' ./internal/gc"
  - criterion: "A cycle that seals into a pause or a continuation keeps its lane and branch"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestSweepSealedCycle_ACycleThatSealsIntoAPauseKeepsItsLaneBranchAndProcesses|TestSweepSealedCycle_ACycleThatSealsIntoAContinuationKeepsItsLaneBranchAndProcesses' ./internal/gc"
  - criterion: "A live lane of another cycle is not touched"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestSweepSealedCycle_TheLiveLaneOfAnotherCycleIsNeverTouched' ./internal/gc"
  - criterion: "A fleet lane cycle that seals through evolve cycle run reaches the sweep"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestRunCycleRun_ASealedFleetLaneCycleHasItsDirtyLaneBranchAndOrphanCleanedAtOnce' ./cmd/evolve"
---

# Eval: gc X10, gc runs when a cycle seals

> Pins the cycle-end step of plan decision D12 (`docs/plans/gc-closes-the-legacy-gap-2026-11.md`, step X10). A sealed cycle has its orphan processes stopped and its lane and branch saved and released at once. A pause, a continuation or a live run keeps them. The sweep must be reached from each path that seals a cycle: the sequential loop, the resume batch and `evolve cycle run`, the path of a fleet lane. Source incident: the cycle 2271 snapshot wired the first two paths only (cycle 2287 TDD found the third).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| sealed-cleaned | dirty lane and orphan are cleaned at once | 4/10 | `go test -run TestSweepSealedCycle_ASealedDirtyLaneWithAnOrphanIsCleanedAtOnce ./internal/gc` |
| hold-keeps | pause and continuation keep the lane | 5/10 | `go test -run TestSweepSealedCycle_ACycleThatSealsInto ./internal/gc` |
| other-cycle-live | the live lane of another cycle stays | 5/10 | `go test -run TestSweepSealedCycle_TheLiveLaneOfAnotherCycleIsNeverTouched ./internal/gc` |
| fleet-lane-path | `evolve cycle run` reaches the sweep | 4/10 | `go test -run TestRunCycleRun_ASealedFleetLaneCycle ./cmd/evolve` |
