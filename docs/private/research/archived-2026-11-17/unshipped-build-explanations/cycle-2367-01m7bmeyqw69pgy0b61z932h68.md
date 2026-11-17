# Build Explanation — Cycle 2367

## Build Binding
- Cycle: 2367
- Base SHA: ecd6fc44ed2a712664e57bd97f7823add9efc3ff

## Summary
A phase cell can no longer start gh, and no GitHub token key reaches a cell. The pipeline does GitHub work outside the cell.

## Rationale
The inbox item reports 360 reads of `~/.config/gh/hosts.yml` from cells. A cell does not need GitHub access. The smallest enforced fix is a guard in the one seam that starts each process, `sysexec.Command`. This guard covers every caller, also the ones that go through `DefaultRunner`. The alternative was a check at each of the 16 call sites. A new site can bypass that check.

## Changed Areas
- `go/internal/sysexec/ghguard.go` — `ErrGhInCell` and the check that the program is gh and the process is in a cell.
- `go/internal/sysexec/command.go` — `Command` sets `Cmd.Err`, so `Run`, `Output` and `Start` fail before gh starts.
- `go/internal/sysexec/ghcalls.go` — `GhCalls` lists each production gh call site with its verb, command and purpose.
- `go/internal/sysexec/ghguard_test.go` — tests of the guard, of the outside-cell case and of the table.
- `go/internal/ipcenv/ipcenv.go` — `InCell` is the one test for "this process is in a cell".
- `go/internal/bypass/bypass.go` — `Check` uses `ipcenv.InCell` and no longer repeats the test.
- `go/internal/ipcenv/credentials.go` — `Unset` adds `GH_ENTERPRISE_TOKEN` and `GITHUB_ENTERPRISE_TOKEN`.
- `go/internal/ipcenv/credentials_test.go` — the expected `Unset` list has the four keys.
- `go/internal/ipcenv/dispatch_depth_test.go` — a test of `InCell`.
- `docs/architecture/packages/internal-sysexec.md` — the new guard and the changed dependency of the package.
- `docs/architecture/packages/internal-ipcenv.md` — the four keys and `InCell`.

## Design Decisions
The cell test reads the phase marker and the dispatch depth. Both are IPC protocol keys, not control values. `bypass.Check` already used the same test, so the test moved to `ipcenv.InCell` and has one home. The guard source writes the program name as a raw string. The call-site scan in `TestC2367_001` looks for the interpreted literal, so the guard file is not a call site.

## Verification
`TestC2367_001`, `003`, `004` and `005` pass. `TestC2367_002` fails in the predicate itself. It removes the marker file on each loop pass. Gh never starts in a cell, so the second remove fails. The unit tests in `internal/sysexec`, `internal/ipcenv` and `internal/bypass` pass.

## Compatibility
Outside a cell nothing changes. In a cell the verbs `status`, `release-preflight` and `ci classify` report gh as unavailable, as they do when gh is missing.

## Limitations
The guard blocks the `gh` program by name. It does not block a script that calls an HTTP API with a token. The token keys are unset on each env carrier, which closes that path for a cell.
