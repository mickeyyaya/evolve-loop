---
score_cap:
  - criterion: "All four production `git worktree add` roots (core.Create, core.CreateFrom, swarm provisioning, the operator CLI) retry a transient rc=255 first-attempt failure and succeed on retry"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestC1590_004_AllFourEntryPointsRetryTransientFailure -tags acs ./acs/cycle1590/"
  - criterion: "The `evolve worktree create` operator CLI routes through the same shared retry helper as the other three roots"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run TestC1590_007_OperatorCLIUsesSharedRetryHelper -tags acs ./acs/cycle1590/"
  - criterion: "When a retryable first attempt fails and a subsequent attempt also fails, the first attempt's diagnostics (rc + stderr) are preserved, not overwritten"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run TestC1590_008_FirstFailurePreservedOnSecondFailure -tags acs ./acs/cycle1590/"
  - criterion: "A permanent failure (e.g. not-a-git-repository, rc=128) fails fast on the first attempt with no backoff"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run TestC1590_009_PermanentFailureFailsFastNoBackoff -tags acs ./acs/cycle1590/"
---

# Eval: Prove or close remaining worktree retry entrypoint coverage

> Pins the worktree-provisioning-retry residuals (`.evolve/inbox/
> 2026-08-03T02-20-00Z-worktree-provisioning-retry.json`, weight 0.82) after
> PR #401 landed the create-root retry: the M1 residual claimed the SAME
> un-retried `git worktree add` still existed at three more roots
> (`core/worktree.go` `CreateFrom`, `swarm/provision.go`'s N-concurrent-worker
> path, `cmd_worktree.go`'s operator CLI) and the M2 residual claimed a second
> attempt's failure masked the first attempt's diagnostics (a SIGKILL'd
> attempt 1 leaves the directory, attempt 2 dies rc=128 already-exists,
> hiding the original transient cause). This eval permanently pins that all
> four production entry points share `gitexec.AddWorktreeWithRetry`, that a
> retryable failure's first-attempt evidence survives a later failure, and
> that a permanent condition still fails loudly with zero backoff (the
> anti-no-op boundary: a helper that always retries — or always swallows —
> would otherwise pass the happy-path checks alone).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| all-roots-retry | 3 of 4 named roots (core Create/CreateFrom, swarm) retry transient failure | 8/10 | `TestC1590_004_AllFourEntryPointsRetryTransientFailure` |
| operator-cli-shared-helper | the 4th root (operator CLI) shares the same retry helper | 6/10 | `TestC1590_007_OperatorCLIUsesSharedRetryHelper` |
| first-failure-preserved | M2 residual: first attempt's rc/stderr survives a 2nd failure | 7/10 | `TestC1590_008_FirstFailurePreservedOnSecondFailure` |
| permanent-fail-fast-negative | permanent condition skips backoff, fails on first attempt | 8/10 | `TestC1590_009_PermanentFailureFailsFastNoBackoff` |
