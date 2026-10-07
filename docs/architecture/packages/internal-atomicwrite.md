# internal/atomicwrite

> Decision context: [ADR-0123](../adr/0123-ledger-durable-evidence-segments-incremental-verify.md) D2 (durable evidence) and the [ledger plan](../../plans/ledger-restructure-2026-10.md). This page keeps the package-level detail.

## Purpose

`internal/atomicwrite` is the one implementation of a crash-safe write: the bytes go to a uniquely named temporary file in the destination's directory, which is then renamed over the target, so a reader sees the old file or the new one and never a half-written truncation. It replaced the half-dozen near-identical `writeJSONAtomic`/`atomicWrite` copies once scattered across packages, and since cycle 1827 also the three durable copies (`storage.writeJSONAtomic` for `state.json` and `cycle-state.json`, the ledger's `writeSegment`, and the evidence store's `writeAtomically`).

## Design

- **Two strengths, one core.** `Bytes(path, data)` writes mode 0644 without fsync; `JSON(path, v)` is `Bytes` of 2-space-indented JSON with no trailing newline. `Durable(path, data, mode)` takes the mode, fsyncs the temporary file before closing it and fsyncs the directory after the rename, so the new name survives a kernel crash once it returns. All three run the same unexported `write` (only the temp pattern and the sync differ), so the durable and plain paths cannot drift.
- **The parent directory is created** (0755) when missing, and the temporary file is removed on every failure before or at the rename. A failure to sync the directory comes after the rename, so the new file is already in place; the error still returns, because the caller asked for durability it did not get.
- **Durable temporaries carry the writer.** `Durable` names its temporary `.<base>.<pid>.<random>.tmp`; `Bytes` keeps `.<base>.<random>.tmp`, which other packages' error goldens normalize. `TempWriter(name)` parses it back to the pid, so a caller that owns a directory (the evidence store) can reap the temporaries of a writer that died between write and rename without touching a live writer's.
- **Seams, not mocks.** `mkdirAll`, `createTemp`, `renameFile`, `removeFile` and `openDir` are package variables; tests swap them to drive every OS-fault branch once here, so each caller collapses to a one-line delegation covered by its own happy path. The `tempFile` and `syncCloser` interfaces are the subset of `*os.File` the algorithm needs.
- **Standard library only**, so any adapter can depend on it without a cycle.

## Invariants

- **The durable contract.** `Durable` writes the exact bytes with the requested mode, handles an empty payload, replaces an existing target (a read-only one too), and leaves no temporary file on a create or rename failure. Pinned by `TestDurable_WritesTheBytesWithTheRequestedMode`, `TestDurable_WritesAZeroLengthPayload`, `TestDurable_ReplacesAnExistingTarget`, `TestDurable_ARenameFailureLeavesNoTempFile` and `TestDurable_ACreateFailureLeavesNoTempFile`.
- **A directory that cannot be synced is an error, never a silent success.** Pinned by `TestDurable_ReportsADirectoryItCannotSync` (a real write-and-search-only directory) and `TestDurable_DirectorySyncFaults` (open, sync and close faults through the seam).
- **Only `Durable` syncs.** Pinned by `TestDurable_SyncError_CleansUpTemp` and `TestBytes_NeverSyncs`.
- **Temporary names parse back to their writer, and nothing else does.** Pinned by `TestTempWriter_ReadsThePidOfTheWriterThatCreatedTheTemp`, `TestDurable_NamesItsTempAfterTheWritingProcess` and `TestTempWriter_RefusesNamesItsWritersNeverCreate`.
- **Every branch runs in a test** (100.0% statement coverage), enforced by ACS `TestC1827_007_AtomicwriteOwnsOneDurableWriterWithEveryFaultBranchTested`; the three moved writers are held to delegating here by `TestC1827_008`–`010` and `TestC1827_012`.

## Findings

- **The durable copies diverged in what they synced.** `storage.writeJSONAtomic` and the ledger's `writeSegment` fsynced the file but never the directory, so a crash right after the rename could lose the new name; only the evidence store synced both. Folding all three into `Durable` gave each the full sequence. Each copy also had its own fault seams (storage's `ioHooks` write/sync/close/rename); those fault cases now live once in this package's tests, and storage keeps only its marshal seam.
