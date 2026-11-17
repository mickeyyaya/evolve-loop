---
score_cap:
  - criterion: "The train runs the batch test before it opens the batch PR, and the production entry point wires it"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestTrainL2_TheBatchTestRunsBeforeTheBatchPROpensAndTheTestedTipLands|TestTrainL2_AShadowStageRunsNoSeamAndLandsAsBefore)$' ./internal/integration && go test -count=1 -run '^(TestIntegrationTrainWiresTheBatchTestAtTheStageOfThePolicy|TestJourneyBatchAtShadowRunsNoBatchTestCommand)$' ./cmd/evolve"
  - criterion: "Two PRs that pass alone and fail together eject the later one with the cause in its brief"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestTrainL2_TwoPRsThatPassAloneAndFailTogetherEjectTheLaterOneWithTheCauseInItsBrief$' ./internal/integration"
  - criterion: "A host kill ejects no member and opens no batch PR"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestHostFailure_|TestRun_)' ./internal/acsrunner && go test -count=1 -run '^(TestTrainL2_AHostKillOfAPackageTestEjectsNoMemberAndOpensNoBatchPR|TestBatchTest_AHostKillOfThePredicateRunEjectsNobodyEvenWithItsSyntheticRed)$' ./internal/integration"
  - criterion: "The L2 pack runs repo-contract, cover-changed, cover-strict, build, vet and gofmt, and a gofmt-only red on the combined tree ejects the member before L3"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestL2Pack_|TestTrainL2_ATreeThatFailsOnlyGofmtOnTheCombinedTreeEjectsTheMemberBeforeTheBatchPR|TestTrainL2_AFormattedMemberPassesTheWholePackAndLands)' ./internal/integration"
---

# Eval: Wire the batch test L2 into the console train

> `BatchTest.Check` had no production caller: the console train landed a batch on CI alone (cycle 2442 scout, inbox item `wave-integration-wire-batch-test-into-the-train`). This eval pins the four acceptance lines. The train calls `Check` before `CI.Open`, a host kill is a typed `acsrunner` failure and never an ejection, and the L2 pack of six checks catches a tree that is red only on gofmt.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| wiring | Check runs before the batch PR; production env wires the seams at the policy stage | 4/10 | `TestTrainL2_TheBatchTestRunsBeforeTheBatchPROpensAndTheTestedTipLands`, `TestIntegrationTrainWiresTheBatchTestAtTheStageOfThePolicy` |
| journey | pass alone, fail together: later member ejected with the cause | 4/10 | `TestTrainL2_TwoPRsThatPassAloneAndFailTogetherEjectTheLaterOneWithTheCauseInItsBrief` |
| host-kill | a kill ejects nobody | 5/10 | `TestHostFailure_*`, `TestRun_*`, `TestTrainL2_AHostKillOfAPackageTestEjectsNoMemberAndOpensNoBatchPR` |
| pack | gofmt-only red caught before L3 | 5/10 | `TestL2Pack_*`, `TestTrainL2_ATreeThatFailsOnlyGofmtOnTheCombinedTreeEjectsTheMemberBeforeTheBatchPR` |
