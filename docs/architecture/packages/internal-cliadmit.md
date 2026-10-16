# internal/cliadmit

## Purpose

`internal/cliadmit` is cross-process admission control for shared LLM CLIs. It enforces a per-CLI cap on concurrent holders across independent evolve loop processes. The holder set is a JSON file under `$XDG_RUNTIME_DIR/evolve/cli-<name>.slots`, guarded by a flock.

## Design

- **Unbounded default.** A cap of zero or less is unbounded. `Acquire` returns at once, creates no slots file and takes no lock, so the behaviour equals the behaviour before the cap existed. The `EVOLVE_CLI_MAX_CONCURRENT_<CLI>` dial is opt-in.
- **Slots path.** `defaultSlotsPath` prefers `$XDG_RUNTIME_DIR` and falls back to `os.TempDir()`. One file per CLI name, so names do not interfere.
- **Release contract.** On success `Acquire` returns a release function. The caller calls it exactly once, with `defer`. On error `Acquire` returns a no-op release, so the caller can always defer without a nil check. The error means "proceed uncapped and warn". A second release call does not panic and does not free a slot twice.
- **Sequence numbers.** `acquireSeq` is an atomic counter. It gives each `Acquire` call a unique `Seq` inside the process, so one PID can hold many distinct slots.
- **Self-healing.** `DefaultTTL` is the longest age of a holder heartbeat before it counts as stale (a crashed process). `Acquire` prunes stale holders each time, so the cap heals itself. `readHolders` treats an absent, empty or unparseable file as an empty set, so a corrupt slots file never blocks new admits for ever.
- **Test seams.** `nowFn` makes the stale-prune tests deterministic. `slotsPathFn` redirects the file.
- **At capacity.** The locked section returns nil and the caller retries with backoff until a slot frees or the context ends. A cancelled context ends the backoff loop.

## Invariants

- A cap of zero or less never touches the file system. Pinned by `cliadmit_test.go`.
- After admission, the slots file holds the holder. After release, it is empty. Pinned by `cliadmit_test.go`.
- The wait loop does not hang on a dead context. Pinned by `cliadmit_amplify_test.go`.

## Findings

- None open.
