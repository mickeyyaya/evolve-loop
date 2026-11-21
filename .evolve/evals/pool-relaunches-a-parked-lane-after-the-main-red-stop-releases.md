---
score_cap:
  - criterion: "a pool relaunches a parked lead marked to resume after its first lane exited"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestPoolResupply_RelaunchesAParkedLeadAfterItsLaneExited$' ./internal/fleet"
  - criterion: "an unmarked todo the pool already launched is never launched again"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPoolResupply_KeepsAnUnmarkedKnownTodoDeduped$' ./internal/fleet"
  - criterion: "a marked todo is never launched while a lane for the same id runs or is pending"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestPoolResupply_NeverRelaunchesAMarkedTodoWhileItsLaneRuns|TestPoolResupply_NeverRelaunchesAMarkedTodoWhileItsLaneIsPending)$' ./internal/fleet"
  - criterion: "the planner marks a released main-red parked scope and the production pool launches it a second time"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestProductionPoolPlanFn_AReleasedMainRedStopRelaunchesTheParkedLeadThePoolAlreadyLaunched$' ./cmd/evolve"
---

# Eval: pool relaunches a parked lane after the main-red stop releases

> The pool keeps every id it launched in a known set, so the resumed lead of a parked lane
> (the same id as its first launch in the same pool run) is dropped at resupply. The fix marks
> the resumed lead and lets a marked todo past the dedupe when no lane for the id runs or is pending.
> Source: inbox item pool-relaunches-a-parked-lane-after-the-main-red-stop-releases, cycle 2582.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| relaunch | marked lead runs a second time | 4/10 | `TestPoolResupply_RelaunchesAParkedLeadAfterItsLaneExited` |
| dedupe kept | unmarked known todo not repeated | 6/10 | `TestPoolResupply_KeepsAnUnmarkedKnownTodoDeduped` |
| no double lane | no relaunch while running or pending | 6/10 | `TestPoolResupply_NeverRelaunchesAMarkedTodo*` |
| production path | planner mark reaches the production pool | 4/10 | `TestProductionPoolPlanFn_AReleasedMainRedStopRelaunches...` |
