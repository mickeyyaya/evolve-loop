# internal/adapters/statemap

## Purpose

`internal/adapters/statemap` is the one lock-owning, full-fidelity read-change-write of `state.json` and of sibling JSON-object state files. The ship phase, the reset seal and the pin repair used to carry their own copies of this read-change-write. They now share this package.

## Design

- **Leaf.** The package imports only `internal/adapters/flock` and the standard library. `core` can import it without a cycle. `storage` was rejected as the home, because `storage` imports `core`.
- **Raw map.** State is a `map[string]any`, not a typed struct. Operator keys that no struct models, such as `expected_ship_sha` and `currentBatch`, must survive a write that touches another key. The typed `storage.UpdateState` covers the orchestrator-modelled subset. This package is the full-fidelity path.
- **Locked update.** `UpdateStateMap` resolves the symlink first. It then holds `flock.PathLock` on the resolved path across the whole read, mutate and write. A worktree writer and a canonical writer therefore contend on one sidecar lock. The mutate function runs under the lock, so it must be fast and must not call `UpdateStateMap` on the same path. A malformed file aborts before the write.
- **Symlink write-through.** `ResolveWriteTarget` follows a bounded chain of links, eight hops at most, and it tolerates a dangling tail. Rename over a link will replace the link with a detached regular file. Writing through to the target keeps the link and every later mutation on the canonical file.
- **Two lineage counters.** `stateRevision` belongs to `storage.UpdateState`. This package only compares it, and it never bumps it. Its own counter, `statemapRevision`, is bumped by `UpdateStateMap`. `WriteStateMap` refuses a write whose counter is below the on-disk value, with `ErrStaleRevision`. The lock serializes writers, and the counters check freshness.
- **Unlocked primitive.** `WriteStateMap` takes no lock. Callers that compose several reads and writes hold the lock themselves. Standalone callers use `UpdateStateMap`.
- **Forensic tripwire.** A write that shrinks `carryoverTodos` by more than half, from more than 20 entries, prints a warning. It never blocks. A key deleted outright counts as a shrink to zero.
- **Format.** The output uses a two-space indent, as `jq` writes it. Diffs against bash-written files therefore stay small.
- **Test seams.** `writeHooks` injects the temp-file syscalls, and `nowFn` injects the clock.

## Invariants

- The lock covers the whole read-mutate-write, so no update is lost across goroutines or processes. Pinned by `TestUpdateStateMap_SerializesConcurrentWriters` and `TestUpdateStateMap_ConcurrentWritersAllLand`.
- A write through a link lands on the canonical file and keeps the link. Pinned by `TestWriteStateMap_PreservesSymlinkAndWritesThrough` and `TestWriteStateMap_DanglingSymlinkCreatesTarget`.
- A worktree writer and a canonical writer share one lock. Pinned by `TestUpdateStateMap_CrossTreeWritersSerializeOnCanonical`.
- A stale writer is refused, and `stateRevision` is never moved by this package. Pinned by `TestWriteStateMap_RefusesStaleRevision`, `TestWriteStateMap_RefusesStaleStatemapRevision` and `TestUpdateStateMap_AutoBumpsOwnCounterAndTimestamp`.
- A malformed file is never overwritten. Pinned by `TestUpdateStateMap_MalformedJSONRefusesToClobber`.
- Unmodelled keys survive an update. Pinned by `TestUpdateStateMap_PreservesUnmodelledFields`.
- The link chain walk is bounded and never errors on a loop or a missing path. Pinned by `TestResolveWriteTarget`.
