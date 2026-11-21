# internal/logread

## Purpose

`internal/logread` is the bounded read path for the logs of one cycle. The command `evolve logs` calls it. An agent that reads a log through this package never receives a whole file. The plan is [logging-and-process-hygiene-2026-10.md](../../plans/logging-and-process-hygiene-2026-10.md) (sections 6.2 to 6.4, step H4).

## Design

- **One `Reader` for each cycle.** `Reader{RunDir, Cycle}` has the verbs `Digest`, `Events`, `Failures`, `Index` and `Show`. Each verb returns lines. A line has `MaxLineBytes` bytes at most.
- **Layers and budgets.** D0 is the digest (40 lines). D1 is the events, the failures and the index (200 lines). D2 is the report of a phase (200 lines). D3 is the raw output of a phase (150 lines).
- **The truncation line.** When a result is longer than its budget, the last line starts with `truncated:` and names a narrower query, for example `evolve logs events --cycle 7 --phase build --from 200`. A short result has no truncation line.
- **The digest.** One line for each `phase.outcome` event. A phase with a verdict other than PASS or SKIPPED has a pointer to `evolve logs events --cycle N --phase P`.
- **`Show` reads a window.** It reads the file line by line and keeps a window from `--from`. A line over `MaxLineBytes` is cut. The rest of that line is read and dropped, so a file of one huge line stays bounded.
- **`log_access` in shadow mode.** `RecordShadow` compares the layer of a read with `phasespec.LogAccess` of the reading phase. For a layer that is not declared it appends `LOG_ACCESS_SHADOW` (WARN, module `log`) to the stream of the cycle. The read goes on. H5 turns the record into a refusal.
- **The prompt block.** `DispatchBlock` gives the digest and one instruction line. The runner adds it only to a phase whose spec declares `log_access`.

## Invariants

- **No verb returns a whole file.** Pinned by `TestC2584_008_NoLogsVerbReturnsAWholeFileLargerThanItsBudget` and `TestC2584_009_AFileOfOneHugeLineIsStillBounded`.
- **A shadow read never blocks.** Pinned by `TestC2584_011_AReadOutsideTheDeclaredAccessRecordsASignalAndDoesNotBlock`.
- **A phase without `log_access` gets the same prompt as before.** Pinned by `TestC2584_015_PromptIsUnchangedWhenThePhaseDeclaresNoLogAccess`.
