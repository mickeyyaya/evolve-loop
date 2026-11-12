# Build Explanation — Cycle 2329

## Build Binding
- Cycle: 2329
- Base SHA: 53bfa07f0a53897b05db025600187e66a9589103

## Summary
The salvage store now holds a verified `refs.bundle` beside `HEAD`, `uncommitted.patch` and `untracked.tgz`. The dev archive writes the same leaf. A failed bundle step keeps the tree and the branch.

## Rationale
Plan decision D1 names one recovery store and adds a verified bundle to it. Decision D2 allows a removal only after `git bundle verify` passes. One shared writer, `writeSalvageLeaf`, serves the cycle salvage and the dev archive. This avoids a second store and a second copy of the layout.

## Changed Areas
- `go/internal/gc/salvage.go` — `salvageWorktree` now calls `writeSalvageLeaf`, which takes an injected git runner. New `bundleVerified` runs create, verify and list-heads, and requires the tip in the listing. `salvageRef` picks `HEAD` for a detached tree.
- `go/internal/gc/branchsweep.go` — `bundleBranch` calls `bundleVerified`, so the branch delete and the salvage share one verification routine.
- `go/internal/gc/devworktrees_apply.go` — `archiveOne` writes the salvage leaf for every archived tree before the checkpoint save and the removal. A failure refuses the archive and writes no record.
- `go/internal/gc/devworktrees_apply_test.go` — the manifest-write test now blocks the archive path, because the evolve directory now holds the leaf.
- `go/internal/gc/salvage_backfill_f1_test.go` — the scripted git answers the bundle step.
- `go/internal/gc/worktrees_test.go` — the scripted git answers `rev-parse` and the bundle verbs.
- `docs/architecture/packages/internal-gc.md` — the package note records D1 and D2.

## Design Decisions
A `gitRun` function type lets the cycle salvage (`WorktreeOptions.git`) and the dev archive (`gitexec.Git`) share one writer. The bundle runs in the tree, which shares the object store of the branch. A rejected alternative: a second writer for the dev archive. It will duplicate the layout.

## Verification
`go test -count=1 ./internal/gc` passes. The scoped coverage run shows 100% for each function of `salvage.go`, `devarchive.go` and `devworktrees_apply.go`. New tests cover each failing bundle step and the detached ref.

## Compatibility
The leaf names and `ListSalvage` output do not change. A salvage leaf gains one file, `refs.bundle`.

## Limitations
This change does not touch detached-tree classification (D5) or the remote sweep.
