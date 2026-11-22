---
score_cap:
  - criterion: "evolve gc check lists each worktree item with its rule and its evidence"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2601_001_' ./acs/cycle2601"
  - criterion: "a hub whose HEAD holds an unpushed merge of a cycle keeps the cycle branch"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2601_004_|TestC2601_005_)' ./acs/cycle2601"
  - criterion: "a failed fetch counts as not landed, even with a stale origin/main ref that holds the merge"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2601_008_|TestC2601_009_)' ./acs/cycle2601"
  - criterion: "evolve gc check mutates nothing and rejects an unknown flag with the usage exit"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^(TestC2601_002_|TestC2601_003_)' ./acs/cycle2601"
  - criterion: "the existing worktree sweep tests pass unchanged"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC2601_007_' ./acs/cycle2601"
---

# Eval: gc check registry, first for the worktree sweep

> Pins the `evolve gc check` report of the worktree sweep and the origin/main landing target.
> The sealed sweep judged "landed" against the hub HEAD, which can hold an unpushed integration merge.
> Source incident: plan PR #1585 (decisions D4, D10, D11), inbox item recycle-x1a-check-registry-worktrees, cycle 2601.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| report-shape | each item carries a rule and evidence | 6/10 | `go test -tags acs -run TestC2601_001_ ./acs/cycle2601` |
| unpushed-merge | the cycle branch stays when only the hub HEAD holds the merge | 8/10 | `go test -tags acs -run 'TestC2601_004_|TestC2601_005_' ./acs/cycle2601` |
| fetch-fail-safe | a failed fetch keeps every branch | 8/10 | `go test -tags acs -run 'TestC2601_008_|TestC2601_009_' ./acs/cycle2601` |
| report-only | the check mutates nothing and rejects a bad flag | 6/10 | `go test -tags acs -run 'TestC2601_002_|TestC2601_003_' ./acs/cycle2601` |
| sweep-unchanged | the existing worktree sweep tests stay green and unedited | 7/10 | `go test -tags acs -run TestC2601_007_ ./acs/cycle2601` |
