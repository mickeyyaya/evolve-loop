---
score_cap:
  - criterion: "A detached tree whose head is in origin/main or equals a pull request head is released, and needs no save"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2299_001' ./acs/cycle2299"
  - criterion: "A detached tree with a unique head is saved before removal, stays when the save fails, and follows the idle rule"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2299_002' ./acs/cycle2299"
  - criterion: "The production gc options wire the pull request head lookup through gh"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2299_003' ./acs/cycle2299"
  - criterion: "Every changed function of internal/gc and cmd/evolve is covered at 100%"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2299_004|TestC2299_005' ./acs/cycle2299"
  - criterion: "The build explanation names decisions D5 and D7"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2299_006' ./acs/cycle2299"
---

# Eval: detached tree rule and live rule of the dev worktree sweep (X4, D5, D7)

> Pins decisions D5 and D7 and component X4 of the plan `docs/plans/gc-closes-the-legacy-gap-2026-11.md`. The dev worktree sweep kept all 21 detached trees with the reason "detached HEAD; no branch keeps its commits". A detached tree on a merged or pull request head holds no unique work, so the sweep releases it. A unique head is saved first. Source incident: the dry run of 2026-11-09 (finding F4 of the plan).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| released-without-save | merged or pull request head is released even when the save fails | 4/10 | `go test -run TestC2299_001` |
| saved-before-removal | a unique head is under a checkpoint ref before the tree goes | 4/10 | `go test -run TestC2299_002` |
| production-wiring | the options of `evolve gc` carry the lookup | 5/10 | `go test -run TestC2299_003` |
| changed-function-floor | each changed function is at 100% | 6/10 | `go test -run TestC2299_004` |
| decision-ids | the explanation names D5 and D7 | 7/10 | `go test -run TestC2299_006` |
