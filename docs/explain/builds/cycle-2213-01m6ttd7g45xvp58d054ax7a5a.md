# Build Explanation — Cycle 2213

## Build Binding
- Cycle: 2213
- Base SHA: d0753767372d3b89bd3f6ee9b5bdd1718e4bbc87

## Summary
`evolve gc` now removes the seat session folders of removed worktrees from the Claude temp root. The report gives the space released.

## Rationale
On 2026-11-06 the Claude temp root held 46.7 GB in 1013 folders. No gc scope read it. A new sweep reuses the existing `gc.temp_ttl_hours` limit, so no new switch exists.

## Changed Areas
- `go/internal/gc/seatsessions.go` — adds `ReapSeatSessions`, `SeatSessionFolderName` and `DefaultSeatSessionRoot`. A folder is removed only when three conditions hold. Its name starts with the worktree base name. No live worktree owns it. No entry inside is newer than the cutoff.
- `go/internal/gc/seatsessions_test.go` — names each new export and covers a missing root and a refused removal.
- `go/internal/gcpolicy/gcpolicy.go` — adds the `seat_session_root` policy field. An empty value selects the default root.
- `go/cmd/evolve/cmd_gc.go` — adds the `seatSessions` step after the pipeline temp step. It prints the count and the space released, or says that the sweep is off when `gc.temp_ttl_hours` is unset.
- `go/cmd/evolve/cmd_gc_seatsessions_test.go` — tests the step through `runGC` with real directories.

## Design Decisions
A live worktree owns a folder when the folder name equals or extends its encoded name with a hyphen. This stops `cycle-1` from keeping `cycle-10`. The check errs toward keeping a folder. Symlinks are never followed or removed.

## Verification
`go test -count=1` passes for `internal/gc`, `internal/gcpolicy` and the `TestRunGC` tests of `cmd/evolve`. All nine ACS predicates (001 to 009) pass.

## Compatibility
The sweep is off when `gc.temp_ttl_hours` is unset, as the pipeline temp sweep is. No existing step changes.

## Limitations
The sweep removes only seat session folders of removed worktrees under the Claude temporary root. Active worktrees and foreign directories are kept.
