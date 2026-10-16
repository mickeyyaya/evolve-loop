# internal/budgethistory

## Purpose

`internal/budgethistory` rolls the timing, cost and token telemetry of the last N cycles into one `Throughput`. The fleet budget layer (`internal/fleetbudget`) sizes a wave against that pace estimate. The package is a leaf. It depends on the two telemetry readers (`phasetiming`, `cyclecost`) and on the standard library.

## Design

- **Duration sizes the wave, not dollars.** The cost of a cycle is $0 on subscription CLIs. That is why the old `--budget-usd` cap was removed as unreliable. So cost cannot size a wave. `MedianCostUSD` is for display only. The honest budget proxy is the speed at which the pipeline turns cycles: the median cycle wall-clock, from which `CyclesPerHour` for one lane follows (`msPerHour / MedianCycleDurationMS`).
- **`Collect` is read-only and degrades.** It walks the same `.evolve/runs/cycle-<n>` workspaces that `soakreport` walks. A cycle that is missing or unparsable is absent evidence and is skipped. It is never an error. A cycle whose phases all took 0 ms has no measurable pace and is skipped, so no divide by zero can occur.
- **The zero `Throughput` means "pace unknown".** With no measurable evidence `Collect` returns the zero value. The caller must read it as unknown and fall back to the policy floor. It is never "zero throughput".
- **`SampleCount` is the weight of the estimate.** It counts the cycles that gave usable timing. A small sample is a weak estimate, and the caller may weigh it.
- **`CostSampleCount` tells "$0 measured" from "cost unknown".** Only a cycle with an event log counts. A cycle with timing and no `*-events.ndjson` (`ErrNoLogs`) makes the cost sample smaller. A caller must read `MedianCostUSD` as "no cost data" when `CostSampleCount` is 0, and may show it as "over N of M cycles".
- **`MedianTokensPerCycle` is the native-unit twin of the duration.** It is the median gross token count of a cycle: input, output, cache read and cache write (`phasetiming.Rollup.TotalTokens`). Tokens are the honest consumption unit on subscription CLIs, so the fleet quota join reads this field. It is observation only: no sizing decision reads it (see `fleetbudget.ShadowJoin`).
- **Gross, not input plus output.** Cache reads and writes bill quota. If you drop them, the burn is understated. The test fixtures give a different median for the input-plus-output definition, so the definition is asserted.
- **A zero token total is absent evidence.** A log from before the `Tokens` field has no token data. The cycle is skipped for the token median. It is not counted as a real zero, because a zero pulls down the median of a mixed group. The value is never filled in from cost or duration.
- **The median uses the midpoint form `lo + (hi-lo)/2`.** The form `(lo+hi)/2` can overflow `int64` on two large adjacent values. The result is the same for `int64` and `float64`. `median` sorts a copy and does not touch the slice of the caller.

## Invariants

- Cost never changes the sizing fields: two batches with equal durations and different costs give the same `MedianCycleDurationMS` and `CyclesPerHour` (`TestCollect_CostCarriedButNotSizing`).
- A genuine $0 is counted in `CostSampleCount`. A cycle with no event log is not (`TestCollect_CostSampleCountDistinguishesNoDataFromZero`).
- A legacy timing file without tokens gives a token median of 0 and still adds its duration (`TestCollect_LegacyTimingWithoutTokensYieldsZeroMedian`).
- The token median uses the gross definition (`TestCollect_MedianTokensPerCycle`).

## Findings

- None recorded.
