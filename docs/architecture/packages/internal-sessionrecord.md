# internal/sessionrecord

## Purpose

`internal/sessionrecord` is the single source of truth for the per-run tmux session registry (concurrency campaign, CB.5). It defines the on-disk JSONL schema that one writer (the bridge driver, at session creation) and one reader (the run teardown, `swarm.ReapRunSessions`) share.

## Design

- **Leaf package.** It imports the standard library only. Bridge and swarm can then both import it without bending the import graph (swarm imports bridge, bridge imports core, and bridge must not import swarm).
- **Run isolation.** The registry lives inside the run workspace (`.evolve/runs/cycle-<N>/`). A run teardown can see only the sessions that its own file records. There is no server-wide listing to mis-match against, which was the class of the 2026-06-11 incident of a reaper that killed sessions of another run.
- **`FileName`.** The name of the registry in a run workspace. `PathIn` gives the path for a workspace directory.
- **`ReapedSuffix`.** The tombstone suffix. `sessionreaper` renames a fully reaped registry to `<registry>` plus this suffix, so every later boot sweep skips that run. The suffix is defined here. The reaper that writes the tombstone and the reader that recovers it (`ReadAllResolving`) then share one definition, and two `.reaped` literals cannot drift apart.
- **`RunScopeToken`.** The run namespace of a session name: "r" and the first 8 ULID characters. The bridge `resolveSession` mints it into `evolve-bridge-r<runid8>-…` names. The observer run-scope assertion (CB.6) refuses sessions without the token when the probe knows its run id. It lives in this leaf so the observer adapter does not need the whole bridge package for one string rule.
- **`Record`.** One created tmux session. The registry is append-only. A session is recorded at creation and never updated, because liveness is the truth of tmux, not of the registry.
- **`Append`.** `O_APPEND` limits the interleaving of concurrent writers of one run (parallel fan-out launches). Line atomicity for regular files is not guaranteed on every OS (macOS in particular). The malformed-line skip of `ReadAll` is the defense for the rare race. It degrades one record to a leak after a crash and never corrupts the record of a neighbor.
- **`Append` close error.** A `Close` error of the writer can mean the line never reached the disk, so `Append` returns it.
- **`ReadAll`.** A missing file is an empty registry (a run that launched no sessions) and not an error. A malformed line is skipped, so the reaper still reaps the well-formed rest of a registry that a crash half wrote. The file handle is read-only, so its `Close` error is not reported.
- **`ReadAllResolving`.** It returns every record that can still be found for a run. It joins the live path with the `.reaped` tombstone and dedups by session id. After `ReapOrphans` renames a fully reaped registry, the live path is gone, so a plain `ReadAll` loses the attribution. This was the accounting hole that Invariant 3 of the fleet soak hit.
- **No made-up attribution.** With neither file present, `ReadAllResolving` returns zero records and no error. Attribution that never existed is not made up, because that is worse than losing it. A relaunched run can record a tombstoned session again, and the dedup drops the copy.

## Invariants

- A registry shows only the sessions of its own run. Pinned by the tests in `sessionrecord_test.go`.
- A malformed line never hides a well-formed neighbor. Pinned by `sessionrecord_test.go`.
- `ReadAllResolving` recovers a tombstoned registry and fabricates nothing. Pinned by `sessionrecord_test.go`.

## Findings

- None open.
