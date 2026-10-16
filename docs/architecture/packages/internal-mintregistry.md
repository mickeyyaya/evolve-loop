# internal/mintregistry

## Purpose

`internal/mintregistry` is the shared per-project record of advisor-minted phase names. It is the cross-lane channel that lets the tree-diff guard tell a concurrent lane's sanctioned mint of `.evolve/phases/<name>/phase.json` from a real deliverable leak.

## Design

- **One writer, one reader.** Only the registrar writes it (the trust-kernel clamp that persists the mint). Only the guard reads it.
- **Anchored to the project root**, not the configured evolve directory. The guard sees only writes under `ProjectRoot`. An exemption anchored anywhere else cannot match a leaked path. `Path` is the single source of the location, `<projectRoot>/.evolve/active-mints.json`.
- **`TTL`** bounds how long a registered mint exempts its path. It must outlast the longest phase window between a lane's baseline snapshot and its post-phase check (well under an hour in practice). It must stay short, so that the exemption does not widen into a standing allowlist. A re-mint refreshes the entry, so an active name never expires mid-batch.
- **Sidecar lock.** `Append` runs under `flock.WithPathLock`. `atomicwrite` replaces the registry's inode by rename, so the data file is never the lock object. `Append` also drops expired entries and replaces a re-minted name in place, which keeps the file bounded.
- **Lock-free reads.** `ActiveNames` takes no lock: the rename means a reader always sees a complete registry, old or new, never a torn one.
- **Corruption.** `Append` resets a corrupt registry loudly, so a bad file does not brick every later mint. `ActiveNames` returns an error and an empty set, so callers fail toward the guard staying armed. `QuarantineCorrupt` renames the file to `<path>.corrupt-<ts>`, so the window without exemptions lasts one check and not until the next mint. It runs under the lock and re-reads first: a concurrent `Append` may have repaired the file, and a healthy or missing registry stays untouched.

## Invariants

- **A corrupt registry fails safe** (empty set plus error). Pinned by `TestActiveNamesCorruptFailsSafeEmpty`.
- **A healthy or missing registry is never quarantined.** Pinned by `TestQuarantineCorrupt_HealthyAndMissingUntouched`.
- **Concurrent appends all survive.** Pinned by `TestAppendConcurrentNamesAllSurvive`.
- **The file is bounded.** Pinned by `TestAppendSameNameReplacesEntry` and `TestAppendPrunesExpiredEntries`.

## Findings

- Cycle 967 aborted falsely: lane 970's mint was charged to lane 967's passing scout, because both lanes diff the same shared tree. The registry exists to prevent that.
