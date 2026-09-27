---
score_cap:
  - criterion: "TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning still PASSES when every pool lane's launch callback is delayed 4s (twice the old 2s budget) before it signals — its correctness no longer rests on a wall-clock bet"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1721_001_BackfillTestToleratesSlowLaneHandoff ./acs/cycle1721/"
  - criterion: "Every test in go/cmd/evolve/cmd_loop_pool_test.go (the backfill test and its five siblings) passes all 50 runs under -race"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1721_002_PoolTestsPassFiftyRaceRuns ./acs/cycle1721/"
  - criterion: "Each wall-clock budget left in the backfill test is a named declaration carrying a comment that explains the margin, not a bare literal"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1721_003_BackfillBudgetIsNamedAndCommented ./acs/cycle1721/"
  - criterion: "No function in cmd_loop_pool_test.go holds a bare-literal wall-clock budget, and all six pool tests are still declared (siblings ruled out, not deleted)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1721_004_NoTestInPoolFileHoldsBareWallClockBudget ./acs/cycle1721/"
---

# Eval: Pool backfill test no longer bets on a 2s wall clock

> Pins the fix for inbox item `fleet-pool-test-wallclock-flake` (filed
> 2026-08-22 after PR #482's CI run 32571232531: ubuntu-latest FAIL,
> macos-latest PASS, same commit). `TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning`
> waited on three bare `time.After(2 * time.Second)` deadlines, so a lane
> that was slow to start on a loaded `-race` + coverage runner was
> indistinguishable from a real scheduling defect. Cycle 1721's
> bug-reproduction phase showed the failure deterministically by delaying the
> lane handoff 2.5s through `go test -overlay`. This eval requires the
> test to tolerate a 4s handoff delay (injected the same way, with no tree
> change), to pass 50 `-race` runs alongside its siblings, and to keep any
> remaining budget as a named, commented declaration, so a later edit cannot
> tighten it back to a bare literal unnoticed.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| slow-handoff-tolerance | Backfill test passes with a 4s injected lane-start delay | 8/10 | `go test -tags acs -run TestC1721_001 ./acs/cycle1721/` |
| race-determinism | All six pool tests pass 50/50 under `-race` | 7/10 | `go test -tags acs -run TestC1721_002 ./acs/cycle1721/` |
| named-commented-budget | No bare-literal budget in the backfill test; the named one is commented | 6/10 | `go test -tags acs -run TestC1721_003 ./acs/cycle1721/` |
| siblings-ruled-out | No bare-literal budget anywhere in the file; no sibling deleted | 6/10 | `go test -tags acs -run TestC1721_004 ./acs/cycle1721/` |
