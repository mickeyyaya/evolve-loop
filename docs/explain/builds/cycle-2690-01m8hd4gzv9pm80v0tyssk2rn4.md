# Build Explanation — Cycle 2690

## Build Binding
- Cycle: 2690
- Base SHA: 92a50e250893e5e3405983fbdf5f47154bc1bca3

## Summary
`integration.SyncToOrigin` and `mainsync.Run` now share one explicit-refspec fetch builder in `internal/gitexec` (`gitexec.FetchMainArgs`), so `SyncToOrigin` updates `refs/remotes/origin/main` even when `remote.origin.fetch` is narrowed or unset.

## Rationale
`integration.SyncToOrigin` previously ran a bare `git fetch origin main`, which leaves `refs/remotes/origin/main` stale when `remote.origin.fetch` omits `main` or is unset. `integration` cannot import `mainsync` because `mainsync` imports `inboxmover`, creating an import cycle. Placing `OriginBranchRefspec` and `FetchMainArgs` in the leaf package `internal/gitexec` removes the duplicate refspec definitions across `mainsync`, `integration.SyncToOrigin`, and `integration.LaneBatcher.cut` with no dependency cycle.

## Changed Areas
- `go/internal/gitexec/fetch.go` — exports `OriginBranchRefspec` and `FetchMainArgs` and updates `FetchOriginBranch` to use `OriginBranchRefspec`, giving all callers one source for the explicit refspec.
- `go/internal/mainsync/mainsync.go` — deletes its local `fetchRefspec` constant and calls `gitexec.FetchMainArgs()`.
- `go/internal/integration/sync.go` — replaces the bare `fetch origin main` call in `SyncToOrigin` with `gitexec.FetchMainArgs()`.
- `go/internal/integration/lanebatcher.go` — deletes its local `originMainRefspec` constant and uses `gitexec.Remote` with `gitexec.OriginBranchRefspec(mainBranch)`.
- `docs/architecture/packages/internal-integration.md` — documents that `SyncToOrigin` uses `gitexec.FetchMainArgs` and `LaneBatcher.cut` uses `gitexec.OriginBranchRefspec`, removing the pending-landing caveat.
- `docs/architecture/packages/internal-mainsync.md` — documents that `mainsync.Run` fetches through `gitexec.FetchMainArgs` shared with `integration.SyncToOrigin`.

## Design Decisions
Placing `OriginBranchRefspec` and `FetchMainArgs` in `internal/gitexec` avoids the `integration -> mainsync -> inboxmover` import cycle. Keeping a private constant in `internal/integration/sync.go` leaves another refspec copy.

## Verification
Targeted tests in `internal/gitexec`, `internal/mainsync`, `internal/integration`, the seven cycle-2690 ACS predicates (`./acs/cycle2690`), and `evolve docs ste-lint --strict` on the updated package docs all pass after the final edit.

## Compatibility
`SyncOutcome` values, error wrapping in `SyncToOrigin`, and `mainsync.Run` behavior remain unchanged for default fetch configurations while fixing narrowed and unset `remote.origin.fetch` configurations.

## Limitations
Two unrelated bare fetch sites outside this lane (`go/cmd/evolve/cmd_boundary_ratchet.go` and `go/internal/marketplacepoll/marketplacepoll.go`) remain unchanged and are tracked in the structure test allowlist.
