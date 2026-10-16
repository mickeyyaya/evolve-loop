# internal/phasetiming

## Purpose

`internal/phasetiming` defines the per-phase latency record that the loop persists to `<workspace>/phase-timing.json`, and the roll-up of that record into a cycle summary. The orchestrator writes `Entry` values. The dossier producer and the `evolve cycle timing` command read them back and call `Rollup`.

## Design

- **Leaf package.** It imports only the standard library, `contextfill` and `cyclestate` (for the `TokenUsage` and `Diagnostic` value types). Core, the dossier and the commands can import it without a cycle.
- **One schema.** Core aliases its phase timing entry type to `Entry`, so the on-disk schema is defined once. The JSON tags are the on-disk contract (ADR-0044, C1). The orchestrator is the only writer.
- **`Read` returns a missing file as `os.ErrNotExist`, not wrapped.** Callers branch on `os.IsNotExist`.
- **Archetype-less entries bucket under `unknown`.** A log written before the archetype field existed still sums to the right total.
- **`BootMS` is coarse.** The tmux driver derives it from a sleep-interval counter. It is therefore a whole number of poll intervals. The readiness baseline is about two seconds, then the steps are one to two seconds.
- It is real but quantized. It is zero (and absent) on the warm, resumed-session and headless paths. It is about 0.5 percent of cycle wall-clock, so it is not a latency lever.
- **Model provenance.** `ModelSource` (`profile`, `pin` or `advisor`) and `ResolvedModel` mirror the per-phase model provenance of the phase response. The dossier reads this file, not the live response.
- **Tokens.** `Tokens` is the usage of the last try, beside `CostUSD`. The detail of each try is in `llm-calls.ndjson`. `Summary` adds the cycle total, the per-archetype buckets, the tokens that `FAIL` entries wasted, and the cache-hit ratio `cache_read / (input + cache_read)`. The ratio is 0 when there was no input.
- **Context fill.** `ContextFillRatio` and `ContextWindowHot` come from the token counts over `contextfill.WindowSizeForTier(ResolvedModel)`. Both are `omitempty`, so an entry whose tier did not resolve emits neither key. A reader can then tell "unknown fill" from a real 0.0. `Rollup` counts hot phases through `contextfill.IsHot`, so the inclusive boundary has one definition. An entry with no ratio is never hot.
- **Diagnostics.** The phase's own severity and message pairs ride on the record. A phase can return `FAIL` by its own classification. An example is the triage rejection of a protected surface.
- It then seals with the reason it gave. The seal and the cycle health report read the error-severity ones. Warnings stay as the durable trail.
- **Legacy logs degrade to absent.** A log written before any of these fields existed parses to zero values and never to an error. A zero is never a fabricated claim.
- **The parallel projection is a shadow estimate.** `ProjectParallelSaving` takes the phases of archetype `evaluate`, except `audit`. It computes the makespan lower bound `max(longest phase, ceil(sum / concurrency))`. `audit` is excluded by name.
- It is the anchor that branches on the verdict and must stay serial. `Entry` carries no field for that. The projection assumes the phases are independent (each reads the same immutable build output). A real ordering edge between two of them makes the saving an upper bound.
- With fewer than two phases, or a concurrency of 1 or less, the saving is 0. It changes no execution.

## Invariants

- **Legacy and unknown stay absent.** `TestEntry_LegacyLogWithoutContextFillParsesToZero`, `TestTimingEntry_LegacyLogParses`, `TestEntry_ContextFillOmittedWhenUnknown` and `TestEntry_DiagnosticsRideTheRecordAndAreOmittedWhenAbsent` pin this.
- **A missing log is `os.ErrNotExist`.** `TestRead_MissingFileIsNotExist` pins it. `TestRollup_LegacyEntriesBucketUnknown` pins the `unknown` bucket.
- **The hot boundary is inclusive.** `TestRollup_HotThresholdBoundaryIsInclusive` pins it.
- **The projection bounds.** The `ProjectParallelSaving` tests pin the makespan bound and the degenerate cases.

## Findings

- None.
