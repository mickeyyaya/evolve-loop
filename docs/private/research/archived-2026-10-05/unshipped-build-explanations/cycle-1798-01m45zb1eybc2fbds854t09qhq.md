# Build Explanation — Cycle 1798

## Build Binding
- Cycle: 1798
- Base SHA: 63d60776ae1f1afcdc27240f80278e440837e2d0

## Summary
`evolve gc` now refuses a bare non-dry run before any reaper touches tmux, and a new `evolve failures` command lists, resets and prunes `state.json:failedApproaches` without hand-editing state.

## Rationale
- **gc ordering.** `runGC` ran the tmux session and socket reapers first and only refused for a missing `--project-root` inside the project sweep, so a refused run had already reaped. Resolving the root up front makes the refusal precede every side effect.
- **failures command.** Operators had only `loop --reset` to clear stale failure entries. A dedicated verb gives a read-only view plus targeted reset and prune, reusing the same class list so the two paths cannot drift.

## Changed Areas
- `go/cmd/evolve/cmd_gc.go` — `runGC` calls `resolveGCProjectRoot` before the reapers and passes the resolved root to the project sweep; the `--project-root` help no longer promises a cwd default.
- `go/cmd/evolve/cmd_failures.go` — new `failures list [--class] [--json]`, `reset [--fingerprint]` and `prune`; list is read-only and returns an empty array for absent state, mutating verbs require `--project-root`, usage errors exit 10.
- `go/cmd/evolve/cmd_loop_maintenance.go` — `resetClasses` is hoisted to a package variable so `loop --reset` and `failures reset` share one list of infrastructure classes.
- `go/cmd/evolve/registry.go` — registers the `failures` subcommand.

## Design Decisions
Reapers were not injected behind a seam; the acceptance predicates drive the real binary with a fake tmux, which proves ordering without a new abstraction. The class list is shared rather than duplicated.

## Verification
`go test -tags acs ./acs/cycle1798` covers 11 predicates (gc ordering, list/reset/prune behaviour, exit codes); `go test -count=1 ./cmd/evolve/` passes, as do gofmt and go vet.

## Compatibility
`evolve gc` without `--project-root` now exits 1 unless `--dry-run` is set. `loop --reset` behaves as before.

## Limitations
No seam-injected unit tests for the gc reapers; ordering is proven end to end only.
