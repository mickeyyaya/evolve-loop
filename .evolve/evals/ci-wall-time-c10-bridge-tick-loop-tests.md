---
score_cap:
  - criterion: "Every pane-loss and TransientDwell tick-loop test passes within its tick ceiling under -race when run alone"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2112_001' ./acs/cycle2112"
  - criterion: "The slow tick-loop tests bound their ticks by a test-scoped threshold and assert the tick count"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2112_002' ./acs/cycle2112"
  - criterion: "No tick-loop test drops a state assertion to run faster"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2112_003' ./acs/cycle2112"
  - criterion: "One test keeps each production threshold of 1200 s and 300 s"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2112_004' ./acs/cycle2112"
  - criterion: "Each reduced tick-loop test still fails when the production behaviour it guards is broken (dwell reset removed, busy guard removed)"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2112_005' ./acs/cycle2112"
---

# Eval: Bridge tick-loop tests reach their state in fewer ticks

> Pins plan component C10 of docs/plans/ci-wall-time-2026-10.md (decision D19). The pane-loss and
> TransientDwell tests loop 150 to 1,200 ticks each through a fake sleep, and each tick costs real
> work under -race. Source incident: CI wall time audit, cycle 2112 inbox item
> ci-wall-time-c10-bridge-tick-loop-tests. Audit round 1 of the same cycle found that one shared
> timeout of 70 s left ResetsOnNonMatchingFrame with 18 matching frames against a 30-observation gate,
> so it passed with the dwell reset removed (H1); the mutation pattern below closes that gap (M1).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tick-ceiling | Slow tests finish under their ceiling, alone, under -race | 6/10 | `TestC2112_001` |
| scoped-threshold | Tick bound comes from a test-scoped value and is asserted | 6/10 | `TestC2112_002` |
| assertion-floor | No state assertion is removed | 7/10 | `TestC2112_003` |
| production-threshold | 1200 s and 300 s stay in one test each | 5/10 | `TestC2112_004` |
| discriminating-power | A reduced test still fails against a mutant of the production seam | 6/10 | `TestC2112_005` |
