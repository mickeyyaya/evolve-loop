# internal/testverdict

> The design of record: [fleet-landing-at-scale-2026-10.md](../../plans/fleet-landing-at-scale-2026-10.md) §5.3 (the cache), decision D3, component S4. The decision: ADR-0134. The key: [internal-testkey.md](internal-testkey.md). The shared writers: [wave-latency-2026-10.md](../../plans/wave-latency-2026-10.md) decision D6, component W5.

## Purpose

`internal/testverdict` stores the green results of package test runs, by the content key of `internal/testkey`. A later scan asks the store if a run with the same inputs was green. A hit lets the scan skip the run.

The store root is a parameter of `NewStore`. `DirFor(projectRoot)` is the one home of the store under a project, and `Store.Root` returns the root of a store. The caller chooses it (for example `.evolve/landing/testcache/`). Component S5 wires it in shadow (`internal/phases/ship/scanshadow.go`): the ship scan reads and writes the store under `<project>/.evolve/landing/testcache/`.

Component W5 adds a second writer. The build floor runs the repo-contract pack of the ship, and it writes its greens to the same store under the gate id `build-floor`. The ship writes under the gate id `ship`. At the stage `enforce` of `gates.scan_cache_gate`, the ship skips a package with a green from any gate on the same key. At the stage `shadow` (the default), nothing skips a run on a hit.

The existing `internal/verdictcache` stays as it is. It caches audit verdicts by tree id, which is another domain.

## Design

- **Green only.** `Put` refuses any outcome except `Pass` with `ErrNotGreen`. A red and a timeout leave no file. A red must always run again, because a stored flake can stick.
- **The gate id.** Each record names the gate that wrote it (`Record.Gate`). `Put` refuses a record with no gate id with `ErrNoGate`. A gate id has 1 to 32 lowercase letters, digits and dashes, and starts with a letter, so it cannot name a path.
- **The layout.** An entry is `<root>/<File>/<Full>.<Gate>.json`. Thus each gate holds its own entry for one content key. `File` and `Full` must be 64-digit lowercase hex, so a key cannot name a path outside the root (`ErrBadKey`). The index entry is `<root>/index/<sha256 of the package>.json`. It holds the latest green record of the package.
- **Two-step lookup.** `Lookup(file, env)` lists the entries under `File`. It returns the first entry whose recorded variables all equal the values in `env`, from any gate. The returned record names its gate, so a mismatch can name the writer. A variable that was set at store time must still be set to the same value. A variable that was not set must still not be set.
- **The log of the last run.** `LastLog(package)` returns the test log of the latest green record. The caller needs it to compute a key, because the names of the variables come from the log.
- **The write.** Each file goes through `internal/atomicwrite`, so a reader sees the old file or the new file. The entry is written first, then the index.
- **The seal.** Each file holds the record and the SHA-256 of the record. A file is a miss when any of these is true:
  - it does not parse, or its sum does not match;
  - its outcome is not `Pass`;
  - its record names another path, or another gate than its file name;
  - its record has no gate id (an entry of the layout before W5);
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

## The parts of the key

The content key of the ship (`internal/testkey`) holds five parts. They are the package, the toolchain (Go version and build variables), the build tags, the test flags and the id of the tree. The gate id is the last part of the identity of a record: one key can hold one entry for each gate. A changed file, toolchain, tag or flag gives a new key, so the package runs again.

## Tiers that are never cacheable

The key sees the files of the tree that `git ls-files --cached --others --exclude-standard` lists. It cannot see a git-ignored file, a file outside the repository, a server, a tmux session or the state of the project. The writer (`internal/phases/ship`) thus never stores two kinds of result:

- **A tagged tier.** A run with any build tag (`integration`, `e2e`, `acs`, `requires_tmux` and the rest) is a tier whose tests reach outside the key. The audit runs only such tiers (the ACS predicates with `-tags acs`, which read the project state, and the CI-parity tier with `-tags integration`). Thus the audit writes no record, and it is not wired to the store.
- **A package that reads the repository tree.** A package that holds a test of `repocontract.TreeReadingTests()` reads files outside the module, some of them ignored by git. The writer never stores it, in any run.

The rule lives in the writer, not in the store, because only the writer knows the tags and the selection of a run.

## The operator verb

`evolve verdicts stats` prints the count of entries for each gate. `evolve verdicts wipe` counts what a wipe removes and changes nothing. `evolve verdicts wipe --apply` empties the store in one step (`Store.Wipe`). Use the wipe when a mismatch shows a key that misses an input (O7 of the research: a poisoned cache must be wiped).

- `Stats()` walks the root. A missing root is an empty store. A directory that it cannot list is an error.
- `Wipe()` runs `Stats()` first, then removes the root. A store that it cannot list is not removed. A file that it cannot remove is an error.

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
- **More writers widen the risk.** The floor and the ship write the same store. A test that reads an input outside the key can pass at the floor and fail at the ship. The shadow signals that case (`SHIP_SCAN_SHADOW_ESCAPE`, with the field `gate`), and the stage stays `shadow` until a wave shows 0 escapes.
- **The writers are not authenticated.** The seal is a plain SHA-256, and the floor runs tests that the builder wrote. A test can write a forged green into the store. At `shadow` this is harmless. At `enforce` the ship skips a package on such a record. The move to `enforce` thus needs an explicit decision on this write path (a security review), in addition to 0 escapes.
- **The key holds no runtime variable.** The writers key with an unknown test log, so `envnames` is empty. A test that reads `CI` or an `EVOLVE_*` variable can pass under one value and fail under another on the same key.
- **No locking.** Two writers of one key write the same content. The rename is atomic, so the last one wins and both are valid.
- **A failed index write.** `Put` returns the error, but the entry file is already in place. A later `Lookup` can hit it.
