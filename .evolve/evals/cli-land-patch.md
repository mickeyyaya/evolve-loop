---
score_cap:
  - criterion: "evolve land --patch F --branch B yields a hub worktree on B at the fetched origin/main tip with the patch applied, nothing committed, nothing pushed"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_001_LandPatchYieldsUncommittedWorktreeOnBranchAtOriginMain$' ./acs/cycle1810"
  - criterion: "evolve land --salvage <leaf> applies the leaf's uncommitted.patch and restores its untracked.tgz files (nested paths included)"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_002_LandSalvageRestoresLeafPatchAndUntrackedFiles$' ./acs/cycle1810"
  - criterion: "a patch conflicting with origin/main exits 1 naming the conflicted file and leaves --3way conflict markers in the worktree"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_003_LandConflictingPatchExits1NamingConflictedFilesWithMarkers$' ./acs/cycle1810"
  - criterion: "landing onto an existing branch exits 1, names it, creates no worktree and leaves the branch where it was"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_004_LandOntoExistingBranchExits1AndCreatesNoWorktree$' ./acs/cycle1810"
  - criterion: "a missing patch file or salvage leaf exits 2 naming it, before any branch or worktree is created"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_005_LandMissingPatchOrLeafExits2BeforeAnySideEffect$' ./acs/cycle1810"
---

# Eval: evolve land — land a patch or a salvaged cycle onto current main

> Pins the `cli-land-patch` inbox item (console, 2026-09-30, core-function CLI inventory),
> first built in cycle 1810. Console work and salvaged cycles land the same way: a branch from
> origin/main, `git apply` of the patch, extraction of `untracked.tgz`, then the commit gate and
> `evolve ship --class manual`. Before this verb those three steps were raw git, where a stale
> base or a partial apply went unnoticed. `evolve land --branch B (--patch F | --salvage <leaf>)`
> creates the dev worktree through the same code as `evolve worktree create --dev`, applies with
> `--3way`, restores a leaf's untracked files, prints the next commands, and never commits or
> pushes. Every check drives the real `evolve` binary against a hub fixture (bare `.repo.git`
> store + runtime plane + origin that moved after the store was cloned), so a stale-base,
> committed, pushed, or partially restored landing fails here.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| core-landing | worktree on B at fetched origin/main, patch applied uncommitted, not pushed | 3/10 | `TestC1810_001_LandPatchYieldsUncommittedWorktreeOnBranchAtOriginMain` |
| salvage-restore | leaf patch applied + untracked files restored | 5/10 | `TestC1810_002_LandSalvageRestoresLeafPatchAndUntrackedFiles` |
| negative-conflict | exit 1, names conflicted file, markers kept | 5/10 | `TestC1810_003_LandConflictingPatchExits1NamingConflictedFilesWithMarkers` |
| negative-existing-branch | exit 1, no worktree, branch untouched | 6/10 | `TestC1810_004_LandOntoExistingBranchExits1AndCreatesNoWorktree` |
| edge-missing-input | exit 2 naming the input, no side effect | 6/10 | `TestC1810_005_LandMissingPatchOrLeafExits2BeforeAnySideEffect` |

## Adversarial Cases

- Negative: a conflicting patch must not report success or silently drop the conflicting hunk.
- Negative: an existing branch must not be reset, moved or checked out.
- Edge/OOD: a salvage archive with nested paths and no directory entries must still extract.
- Cheapest gaming fake: branching from the store's stale local `main` instead of the fetched
  origin tip — `TestC1810_001` fails it because origin moves after the store is cloned.
