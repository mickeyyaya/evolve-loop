# internal/backfill

## Purpose

`internal/backfill` reconstructs a missing phase artifact from `<phase>-stdout.clean.txt`. It covers the case where the `Write` tool call of a phase timed out but the content was still emitted to stdout.

## Design

- **Header table.** `phaseHeaders` maps each phase name to the canonical markdown header that marks the start of its artifact in the clean stdout. The phases are scout, build, audit, tdd, intent, triage, retro and build-planner.
- **`TryExtract`.** It reads `<workspace>/<phase>-stdout.clean.txt` and finds the last occurrence of the header of the phase. It takes the text from that header to the end of the file, trimmed. If that text has at least `minLen` bytes, it writes it to `artifactPath` through `atomicwrite`.
- **Result.** `(true, nil)` means the artifact was written. `(false, nil)` means no header, content too short, or an unknown phase. `(false, err)` means an I/O error on the read or the write.

## Invariants

- **The last header wins.** `TryExtract` starts the artifact at the final occurrence of the header.
- **The write is atomic.** A reader never sees a half-written artifact.
- **An unknown phase or a short extract is not an error.** It is a normal "nothing to recover" result. Only I/O failures are errors.
