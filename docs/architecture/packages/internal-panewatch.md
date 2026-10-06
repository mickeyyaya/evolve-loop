# internal/panewatch

> Design record: [agy liveness and phase watch](../../research/agy-liveness-monitoring-2026-10.md) (component A3). This page keeps the package-level detail.

## Purpose

`panewatch` publishes one liveness snapshot per tmux phase pane, `<workspace>/<agent>-pane-watch.json`.

- **The writer** is the bridge's wait loop, which already captures the pane every 2 s.
- **The readers:**
  - the per-phase observer (`internal/adapters/observer`), which needs progress between the bridge's review checkpoints;
  - the operator view `evolve bridge sessions`, which lists every live pane.

The package imports only the standard library and `internal/atomicwrite`.

## Design

- **One computation, many readers.** The bridge computes each frame once, with the CLI's own pane vocabulary from its manifest, and hands the result to a `Tracker`. A `Frame` carries four values:
  - the normalized progress hash (`panestream.ProgressHash`): spinner frames and chrome removed, so a spinner tick is not progress;
  - busy or idle;
  - the last token line;
  - the footer model label.

  Readers never capture the pane themselves, so they cannot disagree with the bridge about what counts as progress.
- **Progress versus state.**
  - `Tracker.Observe` advances `ProgressAt` only when the hash changes.
  - A change of busy state, token line or model label republishes the snapshot (`UpdatedAt`) without counting as progress.
  - An unchanged frame is not republished, so the file's mtime moves only on a real change.
- **Identity.** The tracker is seeded with what the bridge knows at launch: session, socket, CLI, agent, cycle, run id, the dispatched model token and the writer's pid (`WriterPID`). Readers never parse session names.
- **Atomic writes.** `Write` goes through `atomicwrite.JSON`, so a reader never sees a torn file. `Read` returns `(snapshot, false, nil)` for a missing file and an error for a corrupt one. `ReadAll` lists every snapshot in a workspace in name order, skipping and joining the errors of unreadable ones.
- **Lifetime.** A file on disk does not by itself mean a live pane, because a writer killed mid-wait never gets to remove its snapshot. Three rules close that gap:
  - the bridge removes the snapshot when its wait ends (`Remove`, which tolerates absence);
  - the engine removes `<agent>-pane-watch.json` at the start of every attempt, whatever the driver (`Engine.runAttempt`), so a later attempt of the same agent cannot inherit a dead pane's state;
  - readers ask whether the writer is alive. `Snapshot.WriterAlive(alive)` is false for a missing or dead `WriterPID`, and `ReadLive` returns such a snapshot as absent. The observer and `evolve bridge sessions` both pass `runlease.PIDAlive`, the same pid check the run lease uses.

## Invariants

- **A spinner tick is not progress; new transcript is.** Pinned in the bridge by `TestPaneWatcher_AgySpinnerTickIsBusyWithoutProgress`, against real agy 1.2.17 frames.
- **Only a hash change moves `ProgressAt`.** Pinned by `TestTracker_SameHashIsNotProgressAndNotRepublished` and `TestTracker_BusyTokenOrLabelChangeRepublishesWithoutProgress`.
- **The first frame is always published**, so a reader learns the phase runs on a pane. Pinned by `TestTracker_FirstObservationIsPublishedAsProgress`.
- **The snapshot does not outlive its wait, and a dead writer's snapshot never reads as live.** Pinned by:
  - `TestRunTmuxREPL_RemovesThePaneWatchSnapshotWhenTheWaitEnds`;
  - `TestEngineLaunch_ClearsALeftoverPaneWatchSnapshotBeforeEachAttempt`;
  - `TestPaneWatcher_StampsTheWriterPID`;
  - `TestSnapshot_WriterAliveNeedsALiveRecordedPID` and `TestReadLive_ADeadWritersSnapshotReadsAsAbsent`;
  - `TestCoreAdapter_ADeadWritersSnapshotFallsBackToStdout`;
  - `TestBridgeSessions_ADeadWritersSnapshotIsNotListedAsLive`.
- **A write failure never blocks the phase.** The bridge warns once on stderr and keeps waiting; the observer falls back to stdout and workspace activity. Pinned by `TestPaneWatcher_WriteFailureWarnsOnceAndKeepsWaiting`.

## Findings

- **2026-10 agy liveness work.** The observer had watched the stdout log, which tmux drivers write only at exit, and the workspace mtime, which host writes keep fresh. At the stall threshold it hashed the raw pane, so an animated spinner read alive forever. This package replaced both signals for tmux phases.
