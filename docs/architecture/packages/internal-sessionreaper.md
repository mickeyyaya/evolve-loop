# internal/sessionreaper

## Purpose

`internal/sessionreaper` removes tmux sessions that runs left behind when their lease is no longer fresh. It finds ownership only through each run's session registry, so a sweep cannot target a session that belongs to another run.

## Design

- **Fresh leases prove liveness.** A fresh lease is never reaped. An absent or invalid lease is stale by definition.
- **A bounded sweep.** `DefaultReapTimeout` bounds one sweep, so a wedged tmux socket (a corrupted server, not the common "no server" case) cannot hang its callers. Loop-boot preflight, soak checks and the per-cycle GC all share it.
- **Tombstone, not delete.** After a fully successful reap the registry is renamed with `sessionrecord.ReapedSuffix`. The rename keeps the audit trail, and the existing "no registry, skip" check makes later sweeps skip the run. A relaunched run recreates the registry and becomes sweepable again. The suffix lives in the `sessionrecord` SSOT. `sessionrecord.ReadAllResolving` shares that one definition, and so it can still attribute a reaped session to its run.
- **Best effort.** A failed rename costs one redundant re-sweep.

## Invariants

- **Only a fully successful reap tombstones.** Any killer error leaves the registry in place, so the next sweep retries the run. Pinned by `TestReapOrphans_PartialFailureNotTombstoned`.
- **The skip keys on the per-run marker**, not on process-global or dir-global state. A run reaped between sweeps is still swept. Pinned by `TestReapOrphans_SecondSweepSkipsReapedRuns`.
- **Attribution survives the tombstone.** Pinned by `TestReapOrphans_AttributionDiscoverableAfterTombstone`.
- **The tests never kill a real tmux session.** They inject the killer.

## Findings

- Every loop boot re-reaped the whole run history (4,388 sessions across 304 registries). A fully reaped registry was never marked done. The tombstone fixed it (cycle 769). The soak test then lost sessions after the rename. `ReadAllResolving` fixed that (cycle 806).
