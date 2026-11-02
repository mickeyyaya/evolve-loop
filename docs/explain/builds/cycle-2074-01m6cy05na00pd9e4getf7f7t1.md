# Build Explanation — Cycle 2074

## Build Binding
- Cycle: 2074
- Base SHA: 5011fd5c516bb4b0db34e5cb615d01f66ee43773

## Summary
The quality panel now has a reader for index TQ2, revert sensitivity. It reverts each production hunk of a ship on a scratch worktree. A hunk is sensitive when a test fails without it.

## Rationale
The review of #1103 showed a change whose test still passed after a revert. TQ2 measures that case. The reader follows the shape of the TQ1 reader: an injected runner, a pure scorer and a Wilson interval.

## Changed Areas
- `go/internal/qualitypanel/revert_sensitivity.go` — adds the input, the result, the runner seam and `MeasureRevertSensitivity`.
- `go/internal/qualitypanel/revert_sensitivity_hunks.go` — adds `ParseHunks`, which splits a diff into one patch per production Go hunk.
- `go/internal/qualitypanel/revert_sensitivity_git.go` — adds `GitRevertRunner`, which reverts one hunk in a scratch worktree and runs the package tests.
- `go/internal/qualitypanel/mutation.go` — moves the scratch path check into `checkScratchDirs`, shared by both readers.
- `go/internal/qualitypanel/apicover_named_test.go` — names the new exported symbols in an assertion.
- `docs/architecture/packages/internal-qualitypanel.md` — describes the TQ2 reader.

## Design Decisions
The runner is an interface, so the scorer needs no git. An uncompilable revert counts as sensitive, because a test needs the reverted code. Kill-guard paths are excluded as in the TQ1 reader. TQ2 has no baseline in the registry, so it starts at stage shadow with no code change.

## Verification
The package tests, the git fixture tests and the cycle 2074 predicates pass. Each function of the new files is at 100% statement coverage.

## Compatibility
The existing mutation reader keeps its error text and behavior. No exported symbol is removed.

## Limitations
The reader has no production caller yet. A later component must pass it the ship diff.
