# Build Explanation — Cycle 2168

## Build Binding
- Cycle: 2168
- Base SHA: 09bb331d490eed147e09f5f52c8964ad715e969d

## Summary
The worktree fence now restores the real index for every path that differs from its dispatch state. A file that a seat stages outside its writable list leaves no AD, MM or D status. This holds when the seat undid the stage on disk.

## Rationale
The snapshot used an index copy, so the revert fixed only the working copy. The first fix reset the index only for the paths that the working-copy restore touched. A stage undone on disk then stayed in the index. The fence now records the index entries at dispatch and compares them with the index at the end. This is independent of the working copy.

## Changed Areas
- `go/internal/treefence/fence.go` — `Take` records the index entries. `Restore` writes the differing entries back. `verify` also checks the index. `Begin` no longer writes an index tree, so an unmerged index at dispatch keeps the fence on.
- `go/internal/treefence/retract.go` — `Retract` always calls `Restore`, so a retracted phase gets the index restore.
- `go/internal/treefence/fence_index_test.go` — tests for the staged, undone-on-disk, unstaged-at-dispatch, locked-index and failing-listing cases.
- `go/internal/core/identity_carry_forward.go` — `gatesOnIntactTree` declines the carry when the fence restored any path. The index restore now hides a staged gate write from the later `write-tree` check, so the fence outcome is the signal.
- `go/internal/core/identity_carry_gate_writes_test.go` — tests that a gate write, however the fence reverted it, returns the cycle to Audit.
- `docs/architecture/agent-run.md` — states the index rule of the fence.
- `docs/architecture/packages/internal-treefence.md` — records the design and its pinning tests.

## Design Decisions
A sha256 repository needs a null object id of its own length, so the removal line takes its length from the snapshot tree.

The record keeps the stage number of each entry, so an unmerged path returns to its three stages. A removal line comes before the entries, because git needs a removal before a higher stage. The listing is read-only, so the real index is never rewritten by the snapshot. Writable paths are excluded from the compare, so they stay staged.

## Verification
The twenty-one cycle predicates pass. The treefence package tests pass with the integration tag. Each new or changed function has 100% coverage, except the three functions that had less coverage at the base.

## Compatibility
The public API is unchanged. A failed index listing at `Take` becomes a `TakeErr`, the same warning path as a failed snapshot.

## Limitations
A seat that stages a writable path keeps it staged. A source-writing seat has an inert fence.

A stage that the seat undid on disk changes no working-copy path. The index is restored, but `End` saves a reverted patch only when the working-copy restore lists a path (`go/internal/treefence/fence.go:471`). Such a stage therefore has no reverted patch.

`restoreIndex` writes each entry back through `update-index --index-info` (`go/internal/treefence/fence.go:142`). The record holds the mode, the object and the stage number only. A restored entry therefore loses an intent-to-add flag or a skip-worktree flag that it had at dispatch.
