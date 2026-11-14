# Build Explanation — Cycle 2383

## Build Binding
- Cycle: 2383
- Base SHA: 392339f2bab20b676793a244e8b276e17afe5b5b

## Summary
Branches with commits ahead of `HEAD` and no net diff against their merge-base no longer emit `WorktreeActionFlagUnmerged` in `gc.PlanWorktrees`. As a result, `evolve gc` files zero inbox items for them and keeps the branches intact. Every branch flag item that `evolve gc` does file remains `KindRefState`, which `evolve wave supply` refuses as non-lane-admissible (`console_kind`).

## Rationale
`squashResult.landing()` returns `landingNone` when `NoNet` is true and `Ahead > 0` so deletion safety keeps branches with unmerged commit history. Previously, `scanWorktrees` and `orphanBranchItems` treated every `landingNone` branch as unmerged work and emitted `WorktreeActionFlagUnmerged`. Thus `FileRepeatedFlags` filed inbox items for empty-commit and reverted branches. Recording `NoNet` on `mergedSet` and skipping `WorktreeActionFlagUnmerged` when `isNoNet(branch)` is true stops false flag items without weakening branch deletion safety.

## Changed Areas
- `go/internal/gc/worktrees.go` — tracks `noNet` on `mergedSet` and skips `WorktreeActionFlagUnmerged` for clean no-net branches. Empty-net-diff branches never enter the repeat-flag tally.
- `go/internal/gc/worktrees_squash_test.go` — adds `TestPlanWorktrees_ANoNetBranchWithCommitsAheadIsNeitherFlaggedNorDeleted` covering both an empty-commit orphan branch and a reverted worktree branch.
- `docs/architecture/packages/internal-gc.md` — documents that `PlanWorktrees` keeps no-net branches with commits ahead without emitting `WorktreeActionFlagUnmerged`.

## Design Decisions
- Kept `squashResult.landing()` returning `landingNone` when `NoNet && Ahead > 0` instead of returning `landingNoNet`, because `landingNoNet` marks a branch deletable in `devworktrees_safety_test.go` (`TestSquashResultLanding`).
- Guarded `scanWorktrees` with `!dirty && merged.isNoNet(e.branch)` so a dirty worktree on a no-net branch still receives `WorktreeActionFlagDirty` or `WorktreeActionSalvageRemove`.

## Verification
Targeted `internal/gc` unit tests and all seven cycle-2383 ACS predicates (`TestC2383_001` through `TestC2383_007`) run the real `evolve gc` and `evolve wave supply --json` binaries and pass.

## Compatibility
`PlanWorktrees` continues to preserve branches with `Ahead > 0` and no net diff, and `flagtally.go` continues to emit `KindRefState` for genuine unmerged branches.

## Limitations
Branches with unrelated history (no merge-base with `HEAD`) still return `NoNet: false` from `branchLandedBySquash` and continue to be flagged as unmerged.
