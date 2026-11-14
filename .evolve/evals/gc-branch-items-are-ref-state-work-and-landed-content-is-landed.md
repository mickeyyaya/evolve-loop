---
score_cap:
  - criterion: "gc files no inbox item for a branch whose commit was reverted, proven through the real evolve gc binary"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_001' ./acs/cycle2383"
  - criterion: "gc files no inbox item for a reverted branch that sits behind a moved main"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_002' ./acs/cycle2383"
  - criterion: "gc files no inbox item for a branch whose only commit is empty"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_003' ./acs/cycle2383"
  - criterion: "gc files no inbox item for a worktree-held branch whose tree equals HEAD"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_004' ./acs/cycle2383"
  - criterion: "A branch whose content main already holds still files nothing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_005' ./acs/cycle2383"
  - criterion: "A branch with real unlanded work is flagged on the second run only and is never deleted"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_006' ./acs/cycle2383"
  - criterion: "Every item gc files about a branch is refused as console-owned by evolve wave supply"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2383_007' ./acs/cycle2383"
---

# Eval: gc branch items are ref-state work and landed content is landed

> Pins the contract that `evolve gc` files an inbox item only for a branch that holds real unlanded
> work, and that each item it files is console-owned in `evolve wave supply`. Cycle 2383 found that a
> branch with commits ahead and no net change (a commit and its revert, or an empty commit) still
> reached `FlagUnmerged`, so gc filed a repeat-flag item for content that main already holds.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| revert | gc files nothing for a reverted branch | 4/10 | `TestC2383_001` |
| revert-behind-main | gc files nothing for a reverted branch behind main | 5/10 | `TestC2383_002` |
| empty-commit | gc files nothing for an empty-commit branch | 5/10 | `TestC2383_003` |
| worktree | gc files nothing for a worktree branch with no net diff | 5/10 | `TestC2383_004` |
| landed | gc files nothing for landed content | 6/10 | `TestC2383_005` |
| unlanded | real unlanded work stays flagged and kept | 7/10 | `TestC2383_006` |
| routing | filed items are console-owned in wave supply | 7/10 | `TestC2383_007` |
