# Build Explanation — Cycle 2661

## Build Binding
- Cycle: 2661
- Base SHA: 6dd24b416b9288e5c6fcc7ec70ebfc8509f6a30a

## Summary
The `gc` check registry adds the `branches` check (`branchCheck`), and `evolve gc check` adds `--only <check>`. Under `gc.remote_sweep.stage: enforce`, `evolve gc` deletes owned landed (B1, B2) and grace-expired closed (B3) branches on local and remote sides.

## Rationale
Replacing `otherBranchItems` in `PlanWorktrees` with `PlanBranchSweep` and `ApplyBranchSweep` shares one allow-list, pull request lookup, and salvage record across `evolve gc check --only branches` and `evolve gc`. `PlanWorktrees` keeps `cycle-*` branch management.

## Changed Areas
- `go/internal/gcpolicy/gcpolicy.go` — adds `BranchesPolicy` (`owned_prefixes` and `closed_grace_days`) with `OwnedPrefixList`, `ClosedGrace`, and `IsOwned` defaults.
- `go/internal/gc/gc.go` — exports the `BranchesPolicy` type alias from `gcpolicy`.
- `go/internal/gc/check.go` — registers `branchCheck{}` in `DefaultChecks()` and adds `SelectChecks` for `--only`.
- `go/internal/gc/remotegh.go` — decodes PR `number` and `closedAt` and adds `OpenPullRequests`, `AllPullRequests`, and `BranchPullRequests` with a `1000`-row limit flag.
- `go/internal/gc/remotesweep.go` — adds `CandidatePRAPI` fallback on full PR lookups, `ErrRemoteDeleteFailed`, restore command output, and `Stamp`-gated `branch-gc` writes for live callers; `PlanRemoteSweep` keeps its legacy merged-PR classification.
- `go/internal/gc/branchsweep.go` — implements `PlanBranchSweep`, `ApplyBranchSweep`, `branchCheck`, and `recordBranchDelete` for B1/B2 TSV+bundle and B3 archive refs.
- `go/internal/gc/devlanded.go` — extracts `branchHasOwnWork` so B2 and `devlanded` share one reflog proof helper.
- `go/internal/gc/worktrees.go` — removes the legacy `otherBranchItems` call from `PlanWorktrees`.
- `go/cmd/evolve/cmd_gc_check.go` — adds `--only <check>` to `evolve gc check`.
- `go/cmd/evolve/cmd_gc_remote.go` — runs `PlanBranchSweep` and `ApplyBranchSweep` and reports `ErrRemoteDeleteFailed` as a `WARN` without a non-zero exit code.
- `go/cmd/evolve/cmd_gc.go` — removes the legacy `BranchScopeAll` and `EndedAfter` setup in `project`.
- `go/internal/gc/check_test.go` — updates the `DefaultChecks()` count assertion to include `branches`.
- `go/cmd/evolve/cmd_gc_check_test.go` — updates the `gc check --json` check count assertion to 3.
- `go/internal/gc/branchsweep_test.go` — updates the legacy `otherBranchItems` tests to test `PlanBranchSweep` and `ApplyBranchSweep`.
- `go/cmd/evolve/cmd_gc_branchsweep_test.go` — updates the operator branch-sweep tests for owned prefixes, shadow stage, and `branch-gc` TSV records.
- `go/cmd/evolve/cmd_gc_remote_test.go` — updates remote sweep test fixture branch names to `console/feat-x` to match default owned prefixes.
- `go/cmd/evolve/parallel_serial_tests_test.go` — registers the 14 `TestX11_*` tests from `cmd_gc_branches_x11_test.go` in `serialTests`.
- `go/internal/gc/branchsweep_x11_test.go` — adds unit tests for `branchCheck`, `SelectChecks`, `PlanBranchSweep`, `ApplyBranchSweep`, and `GhRemoteAPI` paths.
- `go/internal/gcpolicy/gcpolicy_x11_test.go` — adds unit tests for `BranchesPolicy` defaults and overrides.
- `go/cmd/evolve/cmd_gc_branches_x11_build_test.go` — adds unit tests for `cmd_gc_remote` error paths and `cmd_gc_check` usage errors.
- `docs/architecture/packages/internal-gc.md` — documents the `branches` check and X11 invariants.

## Design Decisions
Using `gc.remote_sweep.stage` for both local and remote sides avoids adding an unrequested stage key while keeping `shadow` safety. Separating `ErrRemoteDeleteFailed` with a concrete error type lets `evolve gc` warn and continue on GitHub API delete errors while local or bundle safety errors still fail.

## Verification
`TestX11_*` (14/14 in `cmd/evolve`, plus unit tests in `internal/gc` and `internal/gcpolicy`), `TestC2661_001`–`009` (9/9 predicates in `acs/cycle2661`), and targeted `internal/gcpolicy`, `internal/gc`, and `cmd/evolve` tests pass.

## Compatibility
`PlanRemoteSweep`, `ApplyRemoteSweep`, and `ApplyWorktrees` retain their signatures and behavior for zero-value `RemoteSweepOptions` and `WorktreeActionDeleteBranch` callers (`cycle2296`, `cycle2558`). `evolve gc --only` keeps its `cache` and `salvage` scopes. However, `evolve gc` no longer deletes idle "ended" or unowned-prefix non-cycle branches. `go/cmd/evolve/cmd_gc.go` dropped `BranchScopeAll` and `EndedAfter`, and `go/internal/gc/worktrees.go` dropped `otherBranchItems`.

## Limitations
`evolve gc check --only branches` and `evolve gc` share `gc.remote_sweep.stage` for both local and remote delete stages rather than introducing a separate `gc.checks.branches.stage` policy key.
