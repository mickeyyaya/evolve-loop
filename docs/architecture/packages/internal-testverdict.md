# internal/testverdict

> The design of record: [fleet-landing-at-scale-2026-10.md](../../plans/fleet-landing-at-scale-2026-10.md) §5.3 (the cache), decision D3, component S4. The decision: ADR-0134. The key: [internal-testkey.md](internal-testkey.md).

## Purpose

`internal/testverdict` stores the green results of package test runs, by the content key of `internal/testkey`. A later scan asks the store if a run with the same inputs was green. A hit lets the scan skip the run.

The store root is a parameter of `NewStore`. The caller chooses it (for example `.evolve/landing/testcache/`). Component S5 wires it in shadow (`internal/phases/ship/scanshadow.go`): the ship scan reads and writes the store under `<project>/.evolve/landing/testcache/`. Nothing skips a run on a hit.

The existing `internal/verdictcache` stays as it is. It caches audit verdicts by tree id, which is another domain.

## Design

- **Green only.** `Put` refuses any outcome except `Pass` with `ErrNotGreen`. A red and a timeout leave no file. A red must always run again, because a stored flake can stick.
- **The layout.** An entry is `<root>/<File>/<Full>.json`. `File` and `Full` must be 64-digit lowercase hex, so a key cannot name a path outside the root (`ErrBadKey`). The index entry is `<root>/index/<sha256 of the package>.json`. It holds the latest green record of the package.
- **Two-step lookup.** `Lookup(file, env)` lists the entries under `File`. It returns the first entry whose recorded variables all equal the values in `env`. A variable that was set at store time must still be set to the same value. A variable that was not set must still not be set.
- **The log of the last run.** `LastLog(package)` returns the test log of the latest green record. The caller needs it to compute a key, because the names of the variables come from the log.
- **The write.** Each file goes through `internal/atomicwrite`, so a reader sees the old file or the new file. The entry is written first, then the index.
- **The seal.** Each file holds the record and the SHA-256 of the record. A file is a miss when any of these is true:
  - it does not parse, or its sum does not match;
  - its outcome is not `Pass`;
  - its record names another path;
  - its `Key.Full` does not follow from its `File` and `Env`;
  - its key and its log disagree (`testkey.Key.Consistent`).
- **`GC(maxAge)`.** It removes each file whose modification time is older than `maxAge`, then each key directory that is empty. It returns the number of files removed. A missing root removes nothing. A file that another process removed first is not an error. A file that it cannot remove is an error.
- **The post-run key.** The caller computes the key again after each run, with the log of that run, and calls `Put` with that key and that log. `Put` returns `ErrInconsistent` when they disagree, and `ErrBadKey` when `Full` does not follow from the parts.

## The integrity rule

A hit must never hide a red. Three rules enforce it:

1. `Put` stores a green result only.
2. `Lookup` re-checks the outcome of every file that it reads. A damaged or edited file is a miss, so a torn write costs one run and gives no false hit.
3. `Lookup` needs an equal `File` key and equal variable values. A key that the encoding of `internal/testkey` did not produce cannot match.
4. `Put` and `Lookup` check that the key and the log agree. A key from a stale log cannot hide a variable or an outside read.

## Retention

`Trim(root, now, maxAge, maxBytes, remove)` is the retention of the store. It is a function, and it takes the remover as a parameter. `evolve gc` passes a no-op remover for `--dry-run`, so the preview counts the same plan.

- The plan removes each file older than `maxAge`. It then removes the oldest files until the rest fits `maxBytes`.
- It removes the key directories that become empty.
- A failed removal is a report line, not a count.
- `GC` is the older age-only method and has no production caller.

## Known limits

- **A flake can be stored as green.** A run that passed once is stored. The key changes when its inputs change. A later red of the same key is an escape, which component S5 flags.
- **GC uses file age.** The age is the modification time, so a hit does not refresh an entry. Age is a size bound, not a measure of use.
- **Hidden inputs.** The store has the limits of the key. They are in [internal-testkey.md](internal-testkey.md).
- **No locking.** Two writers of one key write the same content. The rename is atomic, so the last one wins and both are valid.
- **A failed index write.** `Put` returns the error, but the entry file is already in place. A later `Lookup` can hit it.
