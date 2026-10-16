# Build Explanation — Cycle 1893

## Build Binding
- Cycle: 1893
- Base SHA: 7a6b8e8a40d7b2819a9ea89b10deec1c9e5c938d

## Summary
The `internal/gittest` package now injects git faults and checks branches for tests. The E3 worktree tests use these helpers and no longer drive the real git by hand.

## Rationale
Each E3 test file had its own PATH shim and its own call of the real git. One owner in `gittest` removes the copies, and the ratchet and predicates can check one place.

## Changed Areas
- `go/internal/gittest/fault.go` — adds `GitFault`, `InstallFault`, `FaultOn` and `BranchExists`. The shim matches the git subcommand, not a substring, so a `show` fault does not catch `show-ref`.
- `go/internal/gittest/fault_args_test.go` — tests the argument filter, the subcommand match, stacked faults, the bypass of `BranchExists`, the input validation and the shell quoting.
- `go/cmd/evolve/cmd_worktree_dev_cleanup_backfill_e3_integration_test.go` — replaces the local `e3FaultyGit` shim with `gittest.InstallFault` and checks the branch with `gittest.BranchExists`.
- `go/cmd/evolve/cmd_worktree_dev_backfill_e3_integration_test.go` — replaces the hand-written `show-ref` shim and the raw `exec.Command(realGit, ...)` with the kit.
- `go/cmd/evolve/cmd_worktree_stale_backfill_e3_test.go` — checks branches with `gittest.BranchExists` instead of `brBranchExists`.

## Design Decisions
- The fault is a `GitFault` value with a verb, optional whole arguments, a skip count, an exit code and fixed output. Synthesis: one value type covers the twelve E3 fault rows; a substring pattern was rejected because it matched `show-ref` for `show`.
- The shim is a `/bin/sh` script that `InstallFault` writes into a test temp directory, so `gittest` imports no test-only helper package.
- `BranchExists` sets `GITTEST_FAULT_BYPASS=1`. Each gittest shim then runs the real git, so a check is true while a fault is active.
- `InstallFault` reads `PATH` through the package variable `getenv = os.Getenv`. An edit to the envtaint allowlist was rejected because that file is a protected control-plane surface outside this task.

## Verification
`go test -count=1 ./internal/gittest/` passes. The four migrated E3 test groups pass under `-tags integration`. `evolve acs suite --cycle 1893` reports the cycle predicates green.

## Compatibility
Production code does not change. Only test helpers and test files change; `brBranchExists` and `bfStub` stay for their other callers.

## Limitations
The subcommand parser skips only the global options `-C`, `-c`, `--git-dir`, `--work-tree` and `--namespace` with a separate value. Other test files keep their raw git calls until a later migration.
