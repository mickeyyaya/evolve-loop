# Build Explanation — Cycle 2475

## Build Binding
- Cycle: 2475
- Base SHA: de22fd9eeaecac66f9b14ffb7631521b3ac0c4d5

## Summary
A phase cell cannot start gh and has no GitHub token. This build adds the gh guard in `sysexec.Command` (`ErrGhInCell`). It adds the shared `ipcenv.InCell` check and two enterprise token keys to the unset list. It adds the inbox curator entry to the gh call table and a document that lists each gh call. Tests and predicates pin each part.

## Rationale
The inbox item reports 360 reads outside grants, mostly `~/.config/gh/hosts.yml` (source: `docs/operations/cell-gh-calls.md:22`, inbox item `cells-read-gh-credentials`, wave 145). The cause is that a cell can start gh, and gh reads its config dir at start. The guard in `go/internal/sysexec/ghincell.go:15` stops gh in a cell, so the config dir is never read. A complete call list makes the decision auditable.

## Changed Areas
- `.evolve/evals/cells-read-gh-credentials.md` — states the score caps of the task.
- `docs/architecture/packages/internal-ipcenv.md` — records the GitHub token keys that the cell env removes.
- `docs/architecture/packages/internal-sysexec.md` — records the gh guard and the call table.
- `docs/operations/cell-gh-calls.md` — lists each gh call with its site, command and purpose.
- `docs/private/research/archived-2026-11-17/superseded-predicate-packages/cycle2367/predicates_test.go` — archives the superseded predicate package of cycle 2367.
- `docs/private/research/archived-2026-11-17/unshipped-build-explanations/cycle-2367-01m7bmeyqw69pgy0b61z932h68.md` — archives the build explanation of the unshipped cycle 2367.
- `go/acs/cycle2475/predicates_test.go` — holds the seven predicates of this task.
- `go/internal/bridge/cell_gh_config_read_test.go` — pins that the host gh config dir is denied in a cell.
- `go/internal/bridge/cell_github_token_test.go` — pins that no GitHub token key reaches a cell.
- `go/internal/bypass/bypass.go` — replaces its inline cell check with `ipcenv.InCell`, so the bypass check and the gh guard share one definition of a cell. Behavior is unchanged.
- `go/internal/ipcenv/credentials.go` — adds `GH_ENTERPRISE_TOKEN` and `GITHUB_ENTERPRISE_TOKEN` to the unset list, so it holds four GitHub token keys.
- `go/internal/ipcenv/credentials_test.go` — tests the token key list.
- `go/internal/ipcenv/dispatch_depth_test.go` — tests the cell markers that the guard reads.
- `go/internal/ipcenv/ipcenv.go` — adds `InCell`, which reports a cell from the phase key or the dispatch depth.
- `go/internal/sysexec/command.go` — sets `cmd.Err` from `refusedInCell`, so a gh command in a cell fails before the process starts.
- `go/internal/sysexec/ghcalls.go` — lists each gh call site, now with the inbox curator.
- `go/internal/sysexec/ghincell.go` — defines `ErrGhInCell` and `refusedInCell`, which returns the error for a gh binary in a cell.
- `go/internal/sysexec/ghincell_test.go` — tests the guard.

## Design Decisions
Removing gh from cells is simpler than granting read access to the config dir. The pipeline does GitHub work outside the cell. The call table is one list in code. The document repeats it, and a predicate ties the two together.

## Verification
`go test -tags acs -count=1 ./acs/cycle2475` passes 7 of 7. The sysexec tests pass. The ste-lint check reports 0 findings for the new document.

## Compatibility
Callers outside a cell are unchanged. A cell that calls gh gets `ErrGhInCell`.

## Limitations
A live wave log is not reproducible in a lane. The operator can confirm the absence of gh config reads at the next wave.
