---
score_cap:
  - criterion: "The sync-main step of each boundary plan completes when the merge brings a policy key that only the merged code reads"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestBoundaryMergedPolicy_TheSyncMainStepOfEachPlanCompletesWhenTheMergeBringsAKeyOnlyTheMergedCodeReads$' ./cmd/evolve"
  - criterion: "Each plan refreshes the code index after sync-main, and the wave plan after the build, through the rebuilt plane binary"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestBoundaryMergedPolicy_(EachPlanRefreshesTheCodeIndexAfterSyncMainAndTheWavePlanAfterTheBuild|TheRefreshStepRunsTheRebuiltPlaneBinaryAndKeepsItsExitCode|TheBoundaryOfAMergedReaderAndItsKeyCompletesWithTheRebuiltBinary|TheStepsThatReadThePolicyStrictlyRunAfterTheBuildInEachPlan|TheBoundaryRunPlanReadsTheMergedPolicyWithTheBinaryTheBuildLeavesOnDisk)$' ./cmd/evolve"
  - criterion: "A policy key that no reader of the merged tree knows stops the boundary at the first step of the rebuilt binary and names the key"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestBoundaryMergedPolicy_(AKeyNoReaderOfTheMergedTreeKnowsStopsTheBoundaryAtTheFirstStepOfTheRebuiltBinaryNamingTheKey|TheStrictLoaderOfTheRefreshVerbStillRefusesAnUnknownKeyByName)$' ./cmd/evolve"
  - criterion: "Every step that reads the policy strictly (inbox curate, selfheal, gc, code refresh) runs after the build, through the rebuilt binary, in both plans"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestBoundaryMergedPolicy_(TheStepsThatReadThePolicyStrictlyRunAfterTheBuildInEachPlan|TheBoundaryRunPlanReadsTheMergedPolicyWithTheBinaryTheBuildLeavesOnDisk|TheBoundaryOfAMergedReaderAndItsKeyCompletesWithTheRebuiltBinary)$' ./cmd/evolve"
  - criterion: "The operations doc records the order of the merge, the build and the refresh"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBoundaryMergedPolicy_(TheOperationsDocRecordsTheOrderOfTheMergeTheBuildAndTheRefresh|TheOperationsDocRecordsThatTheStepsThatReadThePolicyRunAfterTheBuildInBothPlans)$' ./cmd/evolve"
---

# Eval: The boundary parses the merged policy with the rebuilt binary

> Pins the order of the boundary steps after a merge. Source incident: the wave-149 boundary stopped at step 4, because sync-main merged a PR that added the policy key `workflow.builder_selfcheck` with its reader, then refreshed the code index with the pre-merge binary. The strict policy loader of that binary refused the unknown key. The tests run the real sync-main step against a real repository whose merge adds an unknown key.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| merge-completes | sync-main completes when the merge adds a new key | 3/10 | `go test -run TestBoundaryMergedPolicy_TheSyncMainStepOfEachPlanCompletesWhenTheMergeBringsAKeyOnlyTheMergedCodeReads ./cmd/evolve` |
| refresh-order | the refresh runs after the build, through the plane binary | 3/10 | `go test -run TestBoundaryMergedPolicy_TheRefreshStepRunsTheRebuiltPlaneBinaryAndKeepsItsExitCode ./cmd/evolve` |
| unknown-key-fails | a key that no reader knows still stops the boundary by name | 4/10 | `go test -run TestBoundaryMergedPolicy_AKeyNoReaderOfTheMergedTreeKnowsStopsTheBoundaryAtTheFirstStepOfTheRebuiltBinaryNamingTheKey ./cmd/evolve` |
| doc-order | the doc records the order | 6/10 | `go test -run TestBoundaryMergedPolicy_TheOperationsDocRecordsTheOrderOfTheMergeTheBuildAndTheRefresh ./cmd/evolve` |
| strict-steps-after-build | inbox curate, selfheal, gc and code refresh run after the build through the rebuilt binary (audit H1, round 1) | 3/10 | `go test -run TestBoundaryMergedPolicy_TheBoundaryRunPlanReadsTheMergedPolicyWithTheBinaryTheBuildLeavesOnDisk ./cmd/evolve` |
