---
score_cap:
  - criterion: "A PR whose head CI is red is ejected with CauseHeadCI and starts no batch CI"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./internal/integration -run '^(TestTrainPlan_EjectsPRWhoseOwnCIIsRed|TestTrainPlan_EjectsARedHeadBeforeAnyBatchCIStarts)$'"
  - criterion: "A PR whose head CI is pending stays out of the batch and is not ejected"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/integration -run '^TestTrainPlan_KeepsAPendingHeadOutWithoutEjecting$'"
  - criterion: "A batch of green heads plans exactly as before"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 ./internal/integration -run '^TestTrainPlan_ABatchOfGreenHeadsPlansAsBefore$'"
  - criterion: "The batch command wires the head CI check into the production train"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 ./cmd/evolve -run '^TestRunBatch_(EjectsAPRWhoseHeadCIIsRedBeforeTheBatchRuns|DryRunNamesThePRWhoseHeadCIIsRed)$'"
---

# Eval: the integration batch ejects a PR whose own head CI is red

> Pins the head-CI step of `Train.Plan` in `go/internal/integration`. Before it, the boundary batch tested a PR whose own head CI was already red, so one bad PR cost a 3 h bisect (PR #1582). Source incident: inbox record 2026-11-20T10-06-19Z, cycle 2565.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| red-head | ejected with CauseHeadCI, no batch CI | 6/10 | `go test -run TestTrainPlan_EjectsPRWhoseOwnCIIsRed` |
| pending-head | kept out, not ejected | 7/10 | `go test -run TestTrainPlan_KeepsAPendingHeadOutWithoutEjecting` |
| green-heads | plan unchanged | 7/10 | `go test -run TestTrainPlan_ABatchOfGreenHeadsPlansAsBefore` |
| caller-proof | CLI reaches the check | 6/10 | `go test ./cmd/evolve -run TestRunBatch_EjectsAPRWhoseHeadCIIsRedBeforeTheBatchRuns` |
