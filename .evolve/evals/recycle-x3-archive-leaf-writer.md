---
score_cap:
  - criterion: "A failed cycle's branch moves to refs/archive/cycles/<leaf> and evolve archive restore rebuilds branch, patch and untracked files from it"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2619_001' ./acs/cycle2619"
  - criterion: "evolve archive restore rebuilds the branch from the leaf bundle when the archive ref and its objects are gone"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2619_002' ./acs/cycle2619"
  - criterion: "Restore refuses an unknown cycle and an existing branch, and creates nothing"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2619_003|TestC2619_004' ./acs/cycle2619"
  - criterion: "A branch that moved after the scan is not deleted (compare-and-delete against the scanned tip)"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2619_005|TestC2619_006' ./acs/cycle2619"
  - criterion: "The leaf manifest hash verifies, and restore refuses a leaf whose file or manifest no longer matches"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2619_007|TestC2619_008|TestC2619_009' ./acs/cycle2619"
---

# Eval: A cycle archive leaf in the salvage home, and `evolve archive restore`

> Pins the behavior of plan PR #1585 component X3 (decisions D5, D8). The
> salvage leaf gains a hashed manifest, and the failed cycle's branch moves to
> `refs/archive/cycles/<leaf>` with a compare-and-delete. `evolve archive
> restore <cycle>` rebuilds the branch, the patch and the untracked files.
> Source: inbox item `recycle-x3-archive-leaf-writer`, first built in cycle 2576, continued in cycle 2589 and rebuilt for cycle 2593 and re-pinned in cycle 2619.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| round-trip-ref | Branch, patch and untracked files come back from the archive ref | 4/10 | `TestC2619_001` |
| round-trip-bundle | The bundle restores when the ref and objects are gone | 6/10 | `TestC2619_002` |
| restore-negatives | Unknown cycle and existing branch are refused | 6/10 | `TestC2619_003`, `TestC2619_004` |
| compare-and-delete | A moved ref is not deleted | 4/10 | `TestC2619_005`, `TestC2619_006` |
| manifest-hash | The manifest hash verifies and tampering is refused | 5/10 | `TestC2619_007` to `TestC2619_009` |
