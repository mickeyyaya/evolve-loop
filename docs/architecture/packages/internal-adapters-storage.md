# internal/adapters/storage

## Purpose

`internal/adapters/storage` is the filesystem adapter for the `core.Storage` port. It reads and writes the `.evolve/` state files (`state.json`, `cycle-state.json`, and the per-run `run.json` mirror) and owns the locks that serialize those writes.

## Design

- **Cycle-state path.** `cycleStatePath` delegates to `core.ResolveCycleStatePath`, so every reader resolves the fleet per-run override through one resolver.
- **Lossless cycle-state write.** `WriteCycleState` is a read-merge-write under the sidecar lock of `cycle-state.json`. `core.CycleState` does not model the `checkpoint` block, so the write splices the existing block back in. `checkpoint.ApplyToStateFile` holds the same lock, so the two writers never revert each other.
- **Run mirror.** After the global write, `WriteCycleState` mirrors the state to `<WorkspacePath>/run.json`. The mirror is a plain `CycleState` with no checkpoint block, because guards never read the checkpoint and resume reads the global file. A failed mirror is an error: a guard in a worktree that reads a stale phase will fail open.
- **Symlinks.** A cycle worktree links its `state.json` and `cycle-state.json` to the canonical files. Writers lock, read and rename on the resolved target, so the link survives.
- **Lossless state update.** `UpdateState` holds a blocking flock on `state.json.lock` across the whole read-change-write. It bumps `stateRevision` on each write, and it merges the typed view over the raw JSON map. Unmodeled keys pass through verbatim. Modeled keys that the mutation cleared are deleted, not restored from the raw map. `modeledStateKeys` reads the JSON keys of `core.State` by reflection, so the set stays correct as fields are added.
- **Re-entry.** The mutate function of `UpdateState` runs under the cross-process lock. It must not call `UpdateState` again, because the blocking flock will deadlock. A panic in mutate releases the lock.
- **Lock.** `AcquireLock` pairs a process-local flag with `flock(LOCK_NB)`. The flag covers goroutines in one process, because `flock` alone does not exclude same-process callers on macOS and Linux. The mutex guards only the held state. It is not held for the life of the lock, or a second caller will block instead of getting `ErrLockHeld`.
- **Atomic writes.** `writeJSONAtomic` writes a sibling temp file and renames it, so a reader sees the old file or the new one.
- **Test seams.** `hooks` holds the I/O functions. `withHooks` swaps them for one call, and only while the caller holds the package test mutex.

## Invariants

- Cycle-state writers serialize on one sidecar lock, so a checkpoint write and a state write cannot revert each other. Pinned by `TestWriteCycleState_PreservesCheckpoint` and `TestWriteCycleState_CheckpointNotDuplicated`.
- A cycle worktree's state links survive every writer. Pinned by the tests in `statejson_symlink_test.go`.
- The run mirror never carries the checkpoint block, and a mirror failure is an error. Pinned by `statejson_runmirror_test.go`.
- `UpdateState` loses no update across processes, and it keeps keys it does not model. Pinned by `TestUpdateState_TwoProcessStress` and `TestUpdateState_PreservesUnmodeledKeys`.
- A test fixture never writes into a live lane's cycle state. Pinned by `cyclestate_override_scope_test.go`.
- Fleet lanes read and write their own cycle-state file when the per-run override is set. Pinned by `TestCycleState_FleetPerRunOverride_NoClobber`.

## Findings

- The cycle-state write and the checkpoint writer share one lock by path, through `flock.WithPathLock`. The lock is uncontended in the sequential loop.
