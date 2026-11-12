---
score_cap:
  - criterion: "A salvage removal whose bundle create or verify fails keeps the tree and the branch"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^TestApplyWorktrees_ASalvageWhoseBundleStepFailsKeepsTheTreeAndTheBranch$' ./internal/gc"
  - criterion: "A salvage removal stores a verified refs.bundle beside HEAD, the patch and the untracked archive"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestApplyWorktrees_ASalvageRemovalStoresAVerifiedBundleOfTheLaneBranch$' ./internal/gc"
  - criterion: "The dev archive writes the same salvage layout and keeps the tree when the bundle step fails"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestApplyDevWorktrees_(TheArchiveWrites|AClean|AnArchiveWhose)' ./internal/gc"
---

# Eval: gc X1, the salvage store holds a verified bundle

> Pins decisions D1 and D2 of the gc legacy gap plan. gc adds a verified `refs.bundle` to the salvage leaf, and the dev archive writes the same leaf. A failed bundle create or verify keeps the tree and the branch. Source: inbox item gc-legacy-gap-x1-salvage-store-holds-a-verified-bundle, cycle 2329.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| refusal | A failed bundle step keeps the tree and the branch | 3/10 | `go test -run TestApplyWorktrees_ASalvageWhoseBundleStepFailsKeepsTheTreeAndTheBranch` |
| layout | The salvage leaf holds a verified refs.bundle | 4/10 | `go test -run TestApplyWorktrees_ASalvageRemovalStoresAVerifiedBundleOfTheLaneBranch` |
| dev archive | The dev archive writes the same leaf and refuses on a failed bundle | 4/10 | `go test -run TestApplyDevWorktrees_(TheArchiveWrites|AClean|AnArchiveWhose)` |
