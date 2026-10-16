---
score_cap:
  - criterion: "landed.Changes judges a commit's tree through WithCommit without reading the disk, with WithExact demanding byte equality"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1885_001_' ./acs/cycle1885/"
  - criterion: "Prune --landed keeps every delete decision it had: landed, partial, unchanged, baseless and no-origin-main checkpoints"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1885_002_' ./acs/cycle1885/"
  - criterion: "wtcheckpoint.hasLanded calls landed.Changes and keeps no changedPaths walk"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1885_003_' ./acs/cycle1885/"
  - criterion: "The exact-equality decision is recorded in internal-landed.md with its pinning test"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1885_004_' ./acs/cycle1885/"
  - criterion: "No Go line added this cycle carries a comment"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1885_005_' ./acs/cycle1885/"
---

# Eval: checkpoint prune converges on the landed proof

> Pins the second step of the follow-up in `docs/architecture/packages/internal-landed.md`:
> `internal/landed` gains a commit-tree mode, and `evolve checkpoint prune --landed`
> calls it instead of its own path walk. The prune keeps exact byte equality, because
> it deletes a ref. Source: inbox item `checkpoint-prune-converges-on-landed-proof`, cycle 1885.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| commit-tree | WithCommit and WithExact judge a commit | 8/10 | `TestC1885_001_` |
| prune-preserved | no prune decision changes | 8/10 | `TestC1885_002_` |
| wired | hasLanded calls the shared proof | 7/10 | `TestC1885_003_` |
| decision-recorded | exact-equality rule documented | 5/10 | `TestC1885_004_` |
| zero-comments | no added comment | 5/10 | `TestC1885_005_` |
