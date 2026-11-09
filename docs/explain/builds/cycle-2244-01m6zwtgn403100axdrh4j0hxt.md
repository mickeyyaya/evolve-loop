# Build Explanation — Cycle 2244

## Build Binding
- Cycle: 2244
- Base SHA: ff30908c0429dfc822f3dfa0c70160a562a297bf

## Summary
The operator run of `evolve gc` now deletes a local non-cycle branch that is landed or ended. The batch hook still sweeps only `cycle-*` branches.

## Rationale
Plan decision D8 says the hook must never touch the branch of a developer. Decision D2 says a delete follows a verified bundle. A scope value on the sweep options keeps one planner for both callers.

## Changed Areas
- `go/internal/gc/branchsweep.go` — adds the scope type, the planner for other branches, and the bundle-then-delete step.
- `go/internal/gc/worktrees.go` — adds the scope and ended-limit options, the bundle flag on an item, and the call of the new planner. It also moves the checked-out check to one shared function.
- `go/internal/gc/branchsweep_test.go` — covers every changed function of the package at 100%.
- `go/cmd/evolve/cmd_gc.go` — sets the full scope and the ended limit in the operator run only.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the new operator tests as serial, because they swap process seams.
- `docs/architecture/packages/internal-gc.md` — records the new invariant.

## Design Decisions
The zero value of the scope is the cycle scope, so the hook needs no change and no flag. The ended limit reuses `archive_after_days`. A bundle item keeps the delete action, so the operator output still reads `WOULD-DELETE-BRANCH`. The delete uses `update-ref -d` with the planned tip, so a moved tip is refused.

## Verification
The cycle predicates, the gc package tests and the BranchSweep cmd tests pass. The coverage predicate shows 100% for each changed gc function.

## Compatibility
The hook plan is unchanged. The cycle-branch delete path keeps its tests and its behavior.

## Limitations
The sweep does not read open pull requests, so an ended branch with an open pull request is bundled and deleted locally. The remote copy stays.
