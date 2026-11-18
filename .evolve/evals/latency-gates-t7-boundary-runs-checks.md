---
score_cap:
  - criterion: "The boundary plan runs evolve ci latency check and evolve wave latency --check between the quality report and gc"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestBoundarySteps_RunTheT5AndT6LatencyChecksBetweenTheQualityReportAndGC$'"
  - criterion: "A failed latency check prints a WARN and the boundary and the wave launch continue"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^(TestBoundaryRun_AFailedLatencyCheckWarnsAndTheBoundaryContinuesToTheLaunch|TestWaveNext_AFailedLatencyCheckWarnsAndTheWaveLaunches)$'"
  - criterion: "A breach that files an inbox item does not block the boundary"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestBoundaryRun_ABreachThatFilesAnItemDoesNotBlockTheBoundary$'"
  - criterion: "A failure of a step that is not a latency check still stops the boundary"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode$'"
---

# Eval: the boundary of evolve wave next runs the latency checks

> Pins task T7 of docs/plans/latency-gates-2026-10.md section 6. The boundary of evolve boundary run and evolve wave next runs the T5 check (evolve ci latency check) and the T6 check (evolve wave latency --check) as advisory steps between the quality report and gc. A breach files an inbox item and an error prints a WARN, and neither blocks the boundary. Source: cycle 2483.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| plan | Both checks are in both plans, in order | 5/10 | `go test -run TestBoundarySteps_RunTheT5AndT6LatencyChecksBetweenTheQualityReportAndGC` |
| advisory | A failed check warns and the launch continues | 6/10 | `go test -run TestBoundaryRun_AFailedLatencyCheckWarnsAndTheBoundaryContinuesToTheLaunch` |
| breach | A filed breach item does not block | 6/10 | `go test -run TestBoundaryRun_ABreachThatFilesAnItemDoesNotBlockTheBoundary` |
| twin | A hard step still blocks | 7/10 | `go test -run TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode` |
