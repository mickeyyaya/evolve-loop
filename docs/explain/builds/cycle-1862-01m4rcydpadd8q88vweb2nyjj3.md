# Build Explanation — Cycle 1862

## Build Binding
- Cycle: 1862
- Base SHA: 73c878259726a754474200846f21eb60fa98d0c1

## Summary
A dev-worktree archive that has started now runs to its end. When the sweep budget ends during a checkpoint save or a `git worktree remove`, the archive still finishes and writes its manifest row. A tree that the sweep has not yet judged is still deferred and stays untouched.

## Rationale
Before this change, `archiveOne` ran its write steps under the sweep budget. A cut after the checkpoint save left a half-archived tree: a checkpoint ref, but no manifest row (`exists=true, hasRef=true, hasManifest=false`, bug-reproduction report). The archive now gets its own context after the read-only judge passes. The judge stays under the budget, so the budget can still stop a tree before its archive starts.

## Changed Areas
- `go/internal/gc/devworktrees_apply.go` — after the fresh judge accepts the tree, `archiveOne` derives `context.WithTimeout(context.WithoutCancel(ctx), devArchiveTimeout)`. The checkpoint save, the worktree remove and the manifest write run under this context. `devArchiveTimeout` is 2 minutes, so a hung git call still ends.
- `go/internal/gc/devworktrees_budget_test.go` — the TDD phase added three tests and the `archiveStateOf` helper. The build corrected one line of the helper. The helper resolved the path of a tree that was already removed. `filepath.EvalSymlinks` fails on a missing path, so `/var/...` never matched the `/private/var/...` path of the manifest row on macOS. The helper now resolves the parent directory, which still exists. The assertion stays as strict as before.
- `go/acs/cycle1862/predicates_test.go` — the TDD phase wrote the cycle predicates. The build did not change them.
- `.evolve/evals/gc-dev-archive-atomic-under-budget.md` — the TDD phase wrote the eval. The build did not change it.

## Design Decisions
- The dedicated context starts after the judge, not around the whole loop. A loop under `context.WithoutCancel` also archives the next tree after the budget ends. The two-tree test refuses that result.
- The budget check between trees stays where it was: in the judge. A judge under a spent context marks the tree `Skipped`, and `ApplyDevWorktrees` defers it. An extra `ctx.Err()` check at the top of the loop repeats this rule, so it was rejected.
- The fixed timeout is a constant, not a policy key. No caller needs to tune it, and the sweep budget stays the operator's control.

## Verification
- `go test -count=1 ./internal/gc` passes: 220 tests, 0 failures.
- The three new tests fail when `devworktrees_apply.go` is reverted to the base, and pass with the fix.
- `go test -tags acs ./acs/cycle1862` runs the five cycle predicates.

## Compatibility
No exported API changes. The CLI sweep budget (`gc.dev_worktrees.sweep_budget_seconds`) keeps its meaning. A started archive can now run up to `devArchiveTimeout` past the budget.

## Limitations
One archive can run up to 2 minutes past the sweep budget. A checkpoint save that takes longer than 2 minutes still fails, and that tree is kept with its error, as before.
