---
score_cap:
  - criterion: "save --all exits 0 and saves every other worktree when one dev worktree has unmerged index entries"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2663_001|TestC2663_004' ./acs/cycle2663"
  - criterion: "the output names the skipped worktree with a reason and a signal records the skip"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2663_002|TestC2663_003' ./acs/cycle2663"
  - criterion: "a real save failure still exits 1 next to a skip, and the skip ends when the merge ends"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2663_005|TestC2663_006' ./acs/cycle2663"
---

# Eval: checkpoint save --all skips a worktree that is mid-merge

> Pins the contract of `evolve checkpoint save --all` when one dev worktree has unmerged index entries.
> Before the fix, `git write-tree` failed in that worktree and the whole run exited 1.
> Source incident: cycle 2614 build handoff floor; this is cycle 2663.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| exit-and-save | exit 0, other worktrees saved | 4/10 | `TestC2663_001`, `TestC2663_004` |
| report-and-signal | skip named with reason, signal emitted | 5/10 | `TestC2663_002`, `TestC2663_003` |
| no-masking | real failures still exit 1, skip is state driven | 6/10 | `TestC2663_005`, `TestC2663_006` |
