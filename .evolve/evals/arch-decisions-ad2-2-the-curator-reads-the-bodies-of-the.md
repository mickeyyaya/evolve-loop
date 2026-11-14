---
score_cap:
  - criterion: "A Closes-Inbox id in the merged branch of a merge commit is a deterministic consume, and the PR 1220 replay passes"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestMatchLanding_AClosesInboxTrailerInAMergedBranchIsADeterministicConsume|TestCommit_PR1220ReplayAMergeCommitReadsTheClosesIdOfItsMergedBranchBodies|TestCurate_APR1220ReplayThroughTheLandingEntryConsumesTheItem)$' ./internal/inboxcurator"
  - criterion: "Only the merged branch is read; none, Refs-Inbox and unrelated ids consume nothing"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestLanded_AMergeReadsOnlyItsMergedBranchAndNeverTheFirstParentHistory|TestCommit_AnOctopusMergeReadsEveryMergedBranch|TestCommit_AnOrdinaryCommitCarriesTheClosesIdsOfItsOwnBody|TestMatchLanding_ATrailerThatClosesNoOpenItemProposesNothing|TestCurate_AMergedBranchThatDeclaresNoneOrOnlyRefsConsumesNothing)$' ./internal/inboxcurator"
  - criterion: "The Closes-Inbox grammar of a commit and of a merged branch is the grammar of closesinbox, with no second copy in gitlog.go"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^(TestC2374_007|TestC2374_008)' ./acs/cycle2374"
  - criterion: "Every function of landing.go and gitlog.go has 100 percent statement coverage"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC2374_004' ./acs/cycle2374"
  - criterion: "The package note and the spec state the change in STE"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -tags acs -run '^TestC2374_006' ./acs/cycle2374"
---

# Eval: the curator reads the bodies of the merged branch

> Pins AD2-2 of the architecture decisions plan. A merge commit hides the trailer of its branch commits, so the curator missed the landing of PR #1220 and the item `reviewer-profiles-may-write-their-report` stayed open. `Change.Closes` now holds the `Closes-Inbox` ids of the bodies of the merged branch, and a landed id is a deterministic consume. Source: cycle 2374, plan row AD2-2.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| trailer-consume | A landed Closes-Inbox id is a deterministic consume, PR 1220 replay | 4/10 | `go test -run TestMatchLanding_AClosesInboxTrailer ./internal/inboxcurator` |
| scope | Only the merged branch is read, none and Refs-Inbox consume nothing | 6/10 | `go test -run TestLanded_AMergeReadsOnlyItsMergedBranch ./internal/inboxcurator` |
| one-grammar | A fenced example, prose, NONE and a backticked bullet follow closesinbox.IDs | 5/10 | `go test -tags acs -run TestC2374_007 ./acs/cycle2374` |
| coverage | 100 percent statement coverage on the changed files | 7/10 | `go test -tags acs -run TestC2374_004 ./acs/cycle2374` |
| note | The note and the spec state the change in STE | 5/10 | `go test -tags acs -run TestC2374_006 ./acs/cycle2374` |
