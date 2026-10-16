# internal/core/outcome

> Component unit 01 of ADR-0103. The design record: [decomposition unit 01](../decomposition/01-outcome-recorder.md). The chokepoint rule: ADR-0044.

## Purpose

`internal/core/outcome` is the recording chokepoint (C1) for every terminal disposition of a dispatched phase. It records each disposition exactly once, into three places: the cycle result's `PhasesRun`, the phase-timing log, and the `<phase>-usage.json` sidecar. It stamps the end-of-dispatch clock, the phase archetype and the context fill, so no call site reads the clock again or derives the taxonomy again.

## Design

- **Injected collaborators.** The orchestrator injects its clock, its live archetype lookup and its `phase.outcome` emission (the producer keeps its own module). The unit reports its own failure modes as `outcome.warning` signals under module `outcome`.
- **Live reads.** Build the `Recorder` once per orchestrator, after the orchestrator options are applied. The clock is a closure, the archetype lookup is a method value and the Signal Center is an accessor. An option applied later then still takes. A snapshot of the archetype catalog goes stale, because the catalog changes during a cycle.
- **Null Object Center.** A nil accessor, or one that returns nil, means no Center. Only tests do this. The production roots always wire a Center, and `SignalsWired` proves it. A nil Center makes `Emit` a no-op.
- **Order inside `Record`.** It stamps `EndedAt`, `Archetype` and the context fill. It appends to `result.PhasesRun` and to `*timings` (both in place), calls the injected `phase.outcome` hook and then writes the sidecar. The hook runs after the in-memory record and before the sidecar.
- **Empty workspace.** The sidecar write is skipped with a signal. `filepath.Join` on an empty string is relative to the working directory, and it once leaked sidecars into the test tree.
- **Non-finite floats.** A plain struct fails to encode only on a non-finite float, for example a NaN cost from an upstream defect. A write of the nil result truncates the sidecar without a sign. So the failure is a signal and nothing is written. `WritePhaseTimings` handles the same failure the same way and leaves the log untouched.
- **Append-merge timings.** `WritePhaseTimings` writes `phase-timing.json` atomically. Entries on disk (a crashed earlier try, the phases from before a resume) come first, and the entries of this invocation follow. A phase that appears twice is reality, not duplication, because the log records real dispatches. It returns the composed set, and the durable log and both dossier paths project that one set.
- **One composition rule.** `composePhaseTimings` is unexported on purpose. The dossier writers project the set that `WritePhaseTimings` returns and never compose a second one. An absent, unreadable or empty log yields the live set alone.
- **Context fill.** `contextFillFor` keys on `ResolvedModel`, the tier of the terminal try. A value that is not a canonical tier gives an absent fill `(0, false)`. The unit never guesses a window, and the error does not propagate. It is unexported. Only `Record` derives the fill.
- **Sidecar contract.** `UsageSidecar` is the declared on-disk shape of `<phase>-usage.json`: the cost, duration, verdict and stamps of the terminal try. The timing entry of the same dispatch is beside it. A legacy sidecar without `Tokens` parses to zero.

## Invariants

- Every terminal disposition is recorded once, through `Record`. Pinned by `recorder_test.go`.
- A failure to write timings or the sidecar is a signal. It never masks the cycle outcome.
- The sidecar field order is fixed, empty stamps are omitted and the token fields are always present. Pinned by the golden test in `recorder_test.go`.

## Findings

- None open.
