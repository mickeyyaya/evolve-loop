---
score_cap:
  - criterion: "evolve gc mints exactly one inbox item for one branch flagged with, then without, its worktree"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestGC_ABranchFlaggedWithAndThenWithoutItsWorktreeMintsOneInboxItem$' ./cmd/evolve"
  - criterion: "the flag count of a branch survives the loss of its worktree"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2535_002|TestC2535_003' ./acs/cycle2535"
  - criterion: "a tally file with the old key shape keeps its count, for a branch with or without a worktree and for a path-only flag"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2535_004|TestC2535_005|TestC2535_007' ./acs/cycle2535"
  - criterion: "a flag with no branch still keys on its path, and different branches or actions never share a count"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2535_006|TestC2535_008' ./acs/cycle2535"
---

# Eval: gc flag key names the branch only

> Pins the key of the gc flag tally. A branch that loses its worktree between two gc runs must keep one
> tally entry and mint one inbox item, not a second item under a new key. Source incident: the inbox item
> gc-flag-key-names-the-branch-only (cycle 2535 fleet lane).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| entry-point | `runGC` mints one item across the worktree loss | 4/10 | `go test -run TestGC_ABranchFlaggedWithAndThenWithoutItsWorktreeMintsOneInboxItem ./cmd/evolve` |
| count-survives | the count carries across the worktree loss | 5/10 | `go test -tags acs -run 'TestC2535_002\|TestC2535_003' ./acs/cycle2535` |
| legacy-tally | old-shape keys keep their count | 6/10 | `go test -tags acs -run 'TestC2535_004\|TestC2535_005\|TestC2535_007' ./acs/cycle2535` |
| path-and-isolation | path-only flags key on the path; branches and actions stay apart | 6/10 | `go test -tags acs -run 'TestC2535_006\|TestC2535_008' ./acs/cycle2535` |
