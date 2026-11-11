---
score_cap:
  - criterion: "A branch whose merged pull request headRefOid is its tip is landed, and a later commit on it is not"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestPlanDevWorktrees_ABranchWithAMergedPullRequestAtItsTipIsLandedAndALaterCommitIsNot' ./internal/gc/"
  - criterion: "The merged pull request source is read once for each run, and a failed read falls back to git evidence with a note"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run 'TestPlanDevWorktrees_TheMergedPullRequestSourceIsReadOnceForTheWholeRun|TestPlanDevWorktrees_AFailedMergedPullRequestReadFallsBackToGitEvidenceAndSaysSo' ./internal/gc/"
  - criterion: "evolve gc wires the gh source and lands a dev worktree by the merged tip"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run 'TestDevWorktreeGCOptions_TheWiredMergedPullRequestLookup|TestRunGC_DryRunLandsADevWorktreeByItsMergedPullRequestTip' ./cmd/evolve/"
---

# Eval: gc X2, landed means a merged pull request at the tip

> Pins decision D4, component X2 of docs/plans/gc-closes-the-legacy-gap-2026-11.md.
> The train lands a pull request as a new integration commit, so git evidence
> alone misses merged work. A merged pull request whose headRefOid equals the
> branch tip is the landed source; a later commit, another branch name and an
> empty oid are not. Source incident: cycle 2322 inbox item
> gc-legacy-gap-x2-landed-means-a-merged-pr-at-the-tip.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| tip-equality | merged PR at tip lands, later commit does not | 7/10 | `go test -run TestPlanDevWorktrees_ABranchWithAMergedPullRequestAtItsTipIsLandedAndALaterCommitIsNot ./internal/gc/` |
| once-per-run and fallback | one gh read, git fallback with a note | 6/10 | `go test -run TestPlanDevWorktrees_TheMergedPullRequestSourceIsReadOnceForTheWholeRun ./internal/gc/` |
| production caller | evolve gc wires MergedPRs | 7/10 | `go test -run TestRunGC_DryRunLandsADevWorktreeByItsMergedPullRequestTip ./cmd/evolve/` |
