# Build Explanation — Cycle 1858

## Build Binding
- Cycle: 1858
- Base SHA: 189add6fca3232492f45eecfb23cad15a4ec8b45

## Summary
The `remoteOpenPR` helper in `cmd_branches.go` runs the GitHub CLI `gh` without the unsupported `-C` flag. The helper sets the process working directory on the command with `cmd.Dir = dir`. The integration test stub in `cmd_branches_integration_backfill_e1_test.go` and unit tests in `cmd_branches_test.go` verify this argument contract.

## Rationale
The `gh` CLI does not support the `-C` flag for directory selection. A call to `gh -C <dir>` failed with exit status 1 on systems with a configured git remote and `gh` on PATH. When the code sets `cmd.Dir = dir`, `gh` runs in the repository root without invalid flags.

## Changed Areas
- `go/cmd/evolve/cmd_branches.go` — removes the `-C` flag from `gh` arguments and sets `cmd.Dir` to `dir`.
- `go/cmd/evolve/cmd_branches_integration_backfill_e1_test.go` — updates the fake `gh` stub to reject `-C`.
- `go/cmd/evolve/cmd_branches_test.go` — adds unit tests that verify arguments and working directory for `remoteOpenPR`.
- `go/cmd/evolve/parallel_serial_tests_test.go` — registers the four serial `TestRemoteOpenPR` tests that set environment through a helper.
- `.evolve/evals/remote-open-pr-uses-gh-dash-c.md` — records score caps and verification evidence for the task.

## Design Decisions
The code sets `cmd.Dir = dir` directly on `sysexec.Command` before the `Output()` call. This choice keeps the change minimal and avoids edits to external runners.

## Verification
Unit tests in `cmd_branches_test.go` and integration tests in `cmd_branches_integration_backfill_e1_test.go` pass. The three task eval graders pass.

## Compatibility
The signature and return type of `remoteOpenPR` remain unchanged. The fix allows `evolve branches prune` to succeed when run against repositories with active remotes and `gh` installed.

## Limitations
This change only affects `remoteOpenPR` calls to `gh`.
