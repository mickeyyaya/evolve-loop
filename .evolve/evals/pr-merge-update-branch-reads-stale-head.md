---
score_cap:
  - criterion: "mergeOne waits for the new head when the first view after update-branch returns the old head, awaits green on it, and merges"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestPRMerger_WaitsForNewHeadAfterUpdateBranch$' ./cmd/evolve"
  - criterion: "A train where the merge of PR A makes PR B behind merges both PRs in one run"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestPRMerger_TrainWithUpdateBranch$' ./cmd/evolve"
  - criterion: "A head that never moves, an empty head or a failed view inside the wait refuses or exits 2, merges nothing and never awaits CI on the old head"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestPRMerger_UpdateBranchTimeout$' ./cmd/evolve"
  - criterion: "The existing PR merge tests stay green"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestPRMerger_|TestParsePRMergeArgs_|TestRunPR_)' ./cmd/evolve"
---

# Eval: evolve pr merge --update-branch waits for the new head

> This eval pins the fix for inbox bug `pr-merge-update-branch-reads-stale-head`.
> `upToDate` runs `gh pr update-branch` and then reads the PR again at once (`go/cmd/evolve/cmd_pr.go:345-349`).
> GitHub has not moved `headRefOid` yet, so the view returns the old head.
> `awaitGreen` then checks the old head, and `behindBy` refuses it (`cmd_pr.go:321-326`).
> Source incident: the wave 116 to 117 boundary on 2026-10-17, PR #916, before cycle 1925.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| new-head-wait | Waits for the new head, then awaits green and merges | 7/10 | `go test -run TestPRMerger_WaitsForNewHeadAfterUpdateBranch ./cmd/evolve` |
| train-merge | A behind second PR merges in the same run | 7/10 | `go test -run TestPRMerger_TrainWithUpdateBranch ./cmd/evolve` |
| timeout-negative | A lagging, empty or failed head read merges nothing | 6/10 | `go test -run TestPRMerger_UpdateBranchTimeout ./cmd/evolve` |
| regression-clean | The existing PR merge tests pass | 6/10 | `go test -run '^(TestPRMerger_\|TestParsePRMergeArgs_\|TestRunPR_)' ./cmd/evolve` |
