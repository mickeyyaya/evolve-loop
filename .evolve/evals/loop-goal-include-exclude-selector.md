---
score_cap:
  - criterion: "--exclude keeps an excluded id off every lane: seed plan, prior-decision widen and backlog refill"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestPlanSelector_Exclude|TestSelectorFlags_ExcludeNeverAssignsAnExcludedIDToALane' ./internal/loopwave ./cmd/evolve/"
  - criterion: "--only assigns only the listed ids on the seed plan, the prior-decision widen and the backlog refill"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run 'TestPlanSelector_Only|TestSelectorFlags_OnlyAssignsOnlyTheListedIDs|TestSelectorFlags_OnlyAndExcludeTogetherNarrowTheListedIDs' ./internal/loopwave ./cmd/evolve/"
  - criterion: "an --only id that no lane can take exits 10 at launch with GOAL_NAMES_INERT_ITEMS and its reason, whatever the goal text, and a relaunch is not refused"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestSelectorFlags_AnOnlyIDNoLaneCanTake|TestSelectorFlags_TheOnlyCheck|TestSelectorFlags_OnlyIDsTheLanesCanTakeLaunch|TestSelectorFlags_ARelaunchedLaunch' ./cmd/evolve/"
  - criterion: "an id in both --only and --exclude exits 10 naming the id"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestSelectorFlags_AnIDInBothListsExits10NamingTheID' ./cmd/evolve/"
  - criterion: "no inbox file is changed by either flag"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run 'TestSelectorFlags_NoInboxFileChanges' ./cmd/evolve/ && go test -count=1 -run 'TestPlanSelector_PlanningChangesNoInboxFile' ./internal/loopwave"
  - criterion: "the detached child, the chain re-exec and the boundary loop step each carry the same selector"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run 'TestSelectorFlags_TheDetachedChildCarries|TestSelectorFlags_TheChainReExecCarries|TestSelectorFlags_TheBoundaryLoopStepCarries|TestSelectorFlags_TheWaveNextLoopStepCarries' ./cmd/evolve/"
---

# Eval: evolve loop include and exclude selector

> Pins the launch-scoped `--only` and `--exclude` selector of `evolve loop`. Goal text only warns about inbox ids (`refuseInertGoalItems`, cmd_loop_goal_ids.go), so an operator had no way to keep a lane off an item or to confine a launch to a few items without editing the inbox. The selector filters the backlog at the loopwave plan seam (seed, prior-decision widen, refill), fails an unusable selector at launch with exit 10, and rides on all three relaunch paths. Source: inbox item loop-goal-include-exclude-selector, cycle 2463.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exclude-at-the-plan-seam | an excluded id never reaches a lane | 3/10 | `go test -run 'TestPlanSelector_Exclude'` |
| only-at-the-plan-seam | only listed ids reach a lane | 3/10 | `go test -run 'TestPlanSelector_Only'` |
| inert-only-refused | an unusable --only id exits 10 | 4/10 | `go test -run 'TestSelectorFlags_AnOnlyIDNoLaneCanTake'` |
| conflict-refused | an id in both lists exits 10 | 5/10 | `go test -run 'TestSelectorFlags_AnIDInBothLists'` |
| inbox-untouched | no inbox file changes | 5/10 | `go test -run 'TestSelectorFlags_NoInboxFileChanges'` |
| three-argv-paths | detach, chain re-exec and boundary step carry the selector | 4/10 | `go test -run 'TestSelectorFlags_The.*Carries'` |
