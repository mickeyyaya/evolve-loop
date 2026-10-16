# internal/cyclecost

## Purpose

`internal/cyclecost` computes the cost of each phase and of a whole cycle from the unified phase event stream (ADR-0020). It reads `<workspace>/*-events.ndjson`, takes the last `kind == result` envelope of each file, and sums the cost and token counts. The loop command calls it after each cycle to add up the batch cost, which it shows as `total_cost_usd`. This is display-only telemetry. No cost gate reads it, because the token-budget cost gates were removed.

## Design

- **Source format.** Each line is one normalized envelope. The result envelope carries `data.cost_usd` and `data.tokens` with the keys `in`, `out`, `cache_r` and `cache_c` (the shape of `phasestream.Classifier`). The normalizer upstream handles the raw-log quirks, such as a legacy single-blob JSON and a malformed event in a stream. There is no raw fallback here.
- **Typed decode.** The envelope decodes into a struct, not into `map[string]any`. The token counts then read straight into `int64`, without the `float64` that a generic map decode gives to JSON numbers.
- **Cheap pre-check.** Before the parse, a line without the substring `"kind":"result"` is skipped. A false positive (the substring inside a data payload) is harmless, because `ev.Kind == "result"` checks the parsed line again.
- **Long lines.** A result envelope can embed a large payload, so the scanner buffer cap is 16 MB (`maxScannerBufBytes`). A line over the cap makes the phase contribute nothing.
- **The phase name** is the file name without `-events.ndjson`. `subagent.scout.parallel-worker-1-events.ndjson` gives `subagent.scout.parallel-worker-1`.
- **Usage sidecars fill the gaps.** The sidecar `<phase>-usage.json` (ADR-0044, C1) is written at the one recording point of the core. It survives abort paths that never reach the events normalizer of the tmux driver. It adds the phases that have no events file.
- A phase that has an events file keeps that value, because `events.ndjson` is the richer source that is checked line by line. The sidecar names its phase in its own `phase` field, and the pipeline treats that field as the authority, not the file name. The package copies the token field names, not the `cyclestate` type, so it stays a leaf with no core dependency.
- **`token-usage.json` is not a phase.** The bridge writes this global peak-token snapshot (`{"peak_tokens":N}`). It matches the `*-usage.json` glob but has another shape, so it is excluded by name. Otherwise it shows as a phase named `token`.
- **`ParseEventsLog` is the one parser.** The `tokenusage` collector reuses it (token telemetry plan, S2). The token counts that it recovers then agree with this package by construction. Do not fork a second parser.
- **Frozen JSON shape.** `Summary` and `PhaseCost` mirror what `show-cycle-cost.sh --json` produced, so downstream tooling that read that output keeps working byte for byte.
- **Errors.** A zero cost is not an error. A workspace that does not exist is `ErrNoWorkspace`, because the caller failed to enter the cycle, and a sum of zero hides the bug. A workspace with no events file and no sidecar is `ErrNoLogs`. Both mean a cost of $0, but the causes differ.
- **Order is deterministic.** The log list and the sidecar list are sorted, and the phases are sorted by name, so runs agree.
- **Test seams.** `globFn` reaches the glob error branch, which a literal pattern cannot reach. `maxScannerBufBytes` reaches `bufio.ErrTooLong` without a 17 MB fixture. The tests that change either one do not run in parallel.

## Invariants

- **The sidecar fills gaps and never overrides.** `TestCycleCost_PrefersSidecarOverEvents` pins this, and the exclusion of `token-usage.json`.
- **The last result wins.** The two-result stream test in `cyclecost_test.go` pins it. A phase with no result envelope contributes nothing.
- **Billing parity.** `TestSummarizeCycle_ParityViaProduce` and `TestSummarizeCycle_ParityRawVsEvents` pin that the events path recovers the same cost and tokens as the raw result.
- **One parser.** `TestParseEventsLog_ExportedDelegatesToCanonical` pins that the exported function gives the same result as the internal one.

## Findings

- None.
