# Build Explanation — Cycle 2060

## Build Binding
- Cycle: 2060
- Base SHA: f8269cb929be8f9053d677959f8dc2d4d57e981e

## Summary
Each cycle now carries its LLM minutes to the verdict. A failed cycle also carries the fail point, the waste after it and the fail-fast ratio. `evolve wave status` shows them and the wave medians.

## Rationale
The ledger already holds the call times and the signal stream already holds the blocking signals. One pure function joins them, so the arithmetic has one home.

## Changed Areas
- `go/internal/cyclespend/verdict.go` — adds `Verdict` and `VerdictOf`, the one place of the fail-point arithmetic.
- `go/internal/wave/facts.go` — sets `LLMVerdict` for each cycle from the ledger and the first blocking signal.
- `go/internal/wave/measure.go` — carries the verdict of each cycle into the cost facts.
- `go/internal/wavemeasure/cost.go` — adds the two wave medians to the cost.
- `go/cmd/evolve/cmd_wave_status.go` — shows the verdict values on the line of each cycle.
- `go/cmd/evolve/cmd_wave_measures.go` — shows the two medians on the cost line.
- `docs/architecture/packages/internal-cyclespend.md` — documents the verdict measure.
- `docs/architecture/packages/internal-wavemeasure.md` — documents the verdict measure and the JSON keys.

## Design Decisions
The first `gate.rejected` or `phase.outcome` FAIL signal sets the fail point. A shipped cycle has no fail point, even after a corrected rejection.

## Verification
The unit tests of four packages and the cycle 2060 ACS predicates pass.

## Compatibility
The new JSON fields are additive. Existing keys do not change.

## Limitations
A call with no time is never waste. A cycle with no ledger has a zero verdict.
