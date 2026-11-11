# Build Explanation — Cycle 2299

## Build Binding
- Cycle: 2299
- Base SHA: b4778d1451a8ea6bb3b94188f432a71e9b24342f

## Summary
The dev worktree sweep of `evolve gc` now judges a detached tree by its head. A head in `origin/main` holds no unique work. A head equal to the head of a pull request holds none either. The sweep releases such a tree.

Any other head is unique, so the sweep saves it under `refs/checkpoints/` first. This builds component X4 of the gc legacy gap plan. Decision D5 is the detached rule. Decision D7 is the live rule: a merged and clean tree skips the activity rule.

## Rationale
The dry run of 2026-11-09 kept 21 detached trees with the reason "detached HEAD; no branch keeps its commits". Most were review trees on a pull request head. A removal cannot lose the commits of such a tree. Only a head that no ref holds needs a save. The sweep saves it, and then it applies the idle rule of a branch tree.

## Changed Areas
- `go/internal/gc/devworktrees.go` — `gateBranch` judges a detached head (ancestor of main, pull request head, or unique). `DevOptions.PRHeads` and `DevItem.Unique` are new. `archiveReason` names a detached head. `gateOpenPR` skips a tree with no branch.
- `go/internal/gc/devlanded.go` — a merged and clean detached tree skips the activity rule (D7), with no own-work check. A detached worktree has no first reflog entry to compare.
- `go/internal/gc/devworktrees_apply.go` — apply saves a unique head, and pins it when the tree is clean. A failed save or pin keeps the tree.
- `go/internal/wtcheckpoint/save.go` — `PinHead` records a checkpoint that holds the head of a clean tree. `checkName` is shared with `Save`.
- `go/cmd/evolve/cmd_gc_devwt.go` — `ghPullRequestHeads` reads the heads of the pull requests of any state through `gh`.
- `go/cmd/evolve/cmd_loop_outcome.go` — `devWorktreeGCOptions` wires `PRHeads`, so the batch hook and `evolve gc` both carry the lookup.
- `go/internal/gc/devworktrees_test.go` — the test that pinned the old keep of a detached tree is removed.
- `go/internal/gc/devdetached_test.go` — tests of the detached rule in plan and apply.
- `go/internal/gc/devdetached_cover_test.go` — tests of an unreadable main, the lookup note and a failed pin.
- `go/internal/wtcheckpoint/pinhead_test.go` — tests of `PinHead`.
- `go/cmd/evolve/cmd_gc_devwt_prheads_test.go` — tests of the wired lookup.
- `go/cmd/evolve/parallel_serial_tests_test.go` — the two tests that set `PATH` are listed as serial.
- `go/acs/cycle2299/predicates_test.go` — the predicates of the cycle.
- `.evolve/evals/gc-legacy-gap-x4-dev-live-rule-and-detached-rule.md` — the eval graders of the task.
- `docs/architecture/packages/internal-gc.md` — the page records the detached rule.
- `docs/architecture/packages/internal-wtcheckpoint.md` — the page records `PinHead`.

## Design Decisions
- The lookup runs at most once per sweep and never for a tree on a branch. A missing, unavailable or failed lookup makes the head unique, so the sweep saves more and loses no work.
- A pull request head keeps the activity rule. Only a merged and clean head skips it (D7).
- Rejected: a check inside `wtcheckpoint.Save` for a clean tree. `Save` has one meaning (uncommitted work), so `PinHead` is a separate function.
- Rejected: keep each detached tree. It leaves 21 trees with no reason that a removal can lose.

## Verification
The predicates `TestC2299_001` to `TestC2299_006` and the package tests of `internal/gc`, `internal/wtcheckpoint` and `cmd/evolve` pass. The changed functions are covered at 100%.

## Compatibility
The `unique` key of a dev manifest item is new and `omitempty`. An old manifest reads unchanged.

## Limitations
A pull request head is known only when `gh` answers. Without `gh`, a detached head on a pull request counts as unique and is saved first.
