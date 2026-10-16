# internal/adapters/flock

## Purpose

`internal/adapters/flock` is the file-lock primitive that the adapters share. It serializes the short sections that read, change and write of the ledger append and the state update (`Lock`, `PathLock`, `WithPathLock`). It also gives a non-blocking variant (`TryLock`) and the path of the ship lock (`ShipLockPath`).

## Design

- **`Lock` blocks by design.** The storage project lock uses `LOCK_NB` and returns `ErrLockHeld`. The writers that use `Lock` must serialize and never refuse. Every critical section is one read and one write, so the waits are short in practice.
- **Same-process contention.** `flock(2)` locks belong to the open file description. Two opens in the same process contend as well, so `Lock` serializes goroutines and processes alike.
- **Release.** The returned release function unlocks and closes. Call it exactly once, with `defer`. Calling it twice is the concern of the caller.
- **`runtime.KeepAlive`.** The file must outlive the raw-fd syscall. Without `KeepAlive` the garbage collector may finalize and close the file between `Fd()` and the end of the `flock` call. `TryLock` follows the same rule.
- **Test seam.** `flockFn` is the syscall seam, so the tests drive the flock error branches without a second process.
- **`ShipLockPath` has one home.** The path is `<projectRoot>/.evolve/ship.lock`. Every writer that changes the shared main-repo `.git/index` must lock this exact path. The writers are the add, commit, merge and push cluster of the ship integrator and the commit at the close of the cycle dossier. A second string literal of the path reopens the `.git/index.lock` race without a sign. So both call sites resolve it here.
- **`TryLock` refuses.** It tells a caller that another live process owns a resource, so the caller can decline and not queue. It is the primitive for cross-session ownership leases: a second `evolve campaign run` on the same goal hash refuses and does not clobber the first. When another live holder owns the lock, it returns `held=true` with a nil release and a nil error.
- **In-process set.** `heldPaths` records the absolute paths that this process holds through `TryLock`. `flock(2)` alone does not dedup callers of the same process on every platform (the storage project lock adds the same guard). The OS lock covers cross-process exclusion, and the set covers same-process exclusion.
- **Dead owner.** The OS releases the flock when the holder dies, even on SIGKILL, so the lock of a dead owner can be taken at once.
- **Sidecar convention.** `PathLock` locks `<dataPath>.lock`, never the data file. The atomic writers replace the data file by rename, under any descriptor held on it. A lock on the data inode then protects the wrong object after the first write. `state.json`, `cycle-state.json` and the map update of ship all take their lock from `PathLock` and `WithPathLock`. None re-spells `Lock(path + ".lock")`.
- **`WithPathLock`.** It runs a function under `PathLock` and releases the lock on every exit path, a panic included.

## Invariants

- Concurrent `WithPathLock` sections lose no update. Pinned by `withpath_test.go`.
- The lock never opens or truncates the data file. Pinned by `withpath_test.go`.
- A second `TryLock` of the same path in one process reports held on macOS and Linux alike. Pinned by `trylock_test.go`.

## Findings

- None open.
