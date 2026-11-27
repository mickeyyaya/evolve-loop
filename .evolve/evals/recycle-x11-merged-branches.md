---
score_cap:
  - criterion: "evolve gc check --only branches plans a delete for the merged branch (B1) and the closed branch past grace (B3), keeps the open, checked-out, unowned, cycle-* and empty new branches, and prints the allow-list with tip, pull request and class"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestX11_CheckPlansTheMergedAndTheClosedBranchAndKeepsEveryOther|TestX11_CheckPrintsTheAllowListWithTipPullRequestAndClass|TestX11_CheckOnlyTakesTheNamesOfTheRegistryAndGCOnlyKeepsItsScopes)$' ./cmd/evolve"
  - criterion: "a closed pull request that the branch moved past is kept, and a branch whose tip moves after the plan stays after evolve gc"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestX11_AClosedBranchThatMovedPastItsPullRequestIsKept|TestX11_ABranchWhoseTipMovesAfterThePlanStaysAfterGC)$' ./cmd/evolve"
  - criterion: "the default branch that origin/HEAD names stays even when it is not main"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestX11_TheDefaultBranchThatOriginHeadNamesStays$' ./cmd/evolve"
  - criterion: "a pull request lookup that returns exactly its limit falls back to one lookup per candidate branch"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestX11_AFullPullRequestLookupFallsBackToOneLookupPerCandidate$' ./cmd/evolve"
  - criterion: "each delete writes its record and the printed restore command rebuilds a local and a remote branch; B3 leaves refs/archive/branches/<stamp>/<branch>"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestX11_GCRecordsEachDeleteAndThePrintedRestoreRebuildsBothSides$' ./cmd/evolve"
  - criterion: "the shadow stage prints WOULD-DELETE and deletes nothing, and a failed remote delete is a WARN that keeps the gc exit code"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^(TestX11_ShadowPrintsWouldDeleteAndDeletesNothing|TestX11_AFailedRemoteDeleteIsAWarnAndTheExitCodeStays)$' ./cmd/evolve"
  - criterion: "a cycle-* branch is never planned and a closed branch is planned only past the grace"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestX11_ACycleBranchIsNeverPlannedByThisCheck|TestX11_AClosedBranchIsPlannedPastTheGraceAndKeptBefore)$' ./cmd/evolve"
---

# Eval: the branches gc check

> Pins the contract of the registry check `branches` and of the delete path of `evolve gc` for landed and closed branches (plan section 13, decisions D17 to D23, component X11). Source incident: on 2026-11-22 the hub held 130 local branches outside origin/main, the remote sweep never deleted, and no verb printed the allow-list. The tests build a fixture hub with a bare origin and a fake `gh`, and drive the real verb through `runGC`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| allow-list | the plan names the delete and keep cases | 6/10 | `go test -run TestX11_CheckPlans...` |
| moved tip | a branch past its closed pull request or its plan stays | 7/10 | `go test -run TestX11_ABranch...` |
| default branch | origin/HEAD decides, not a hard-coded name | 7/10 | `go test -run TestX11_TheDefaultBranch...` |
| lookup limit | a full lookup falls back per candidate | 7/10 | `go test -run TestX11_AFullPullRequestLookup...` |
| record and restore | a delete leaves a record that restores it | 8/10 | `go test -run TestX11_GCRecords...` |
| stage and warn | shadow deletes nothing, a failed remote delete warns | 8/10 | `go test -run TestX11_Shadow...` |
| cycle and grace | cycle-* is never planned, grace is honoured | 7/10 | `go test -run TestX11_ACycleBranch...` |
