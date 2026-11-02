# Build Explanation — Cycle 2099

## Build Binding
- Cycle: 2099
- Base SHA: 3defd2ef106514651164d54d58f18a156e11d3cb

## Summary
Each cycle result now carries the LLM minutes to verdict, the fail point, the waste after the fail point and a fail-fast ratio. The wave status command lists these values for each cycle and the wave median.

## Rationale
The first blocking signal of a failed cycle marks the fail point. The LLM time after that signal is waste. The ratio of fail point to total is the fail-fast ratio. A shipped cycle gets minutes to ship and no fail point. This reuses the LLM ledger and the signal log that the wave reader already loads, so no new data source exists.

## Changed Areas
- `go/internal/cyclespend/verdict.go` — new `Verdict` type and `VerdictOf` function that compute minutes to verdict, fail point, waste and ratio from the LLM call records.
- `go/internal/wave/facts.go` — `Cycle` carries `LLMVerdict`. `readVerdict` fills it, and `applySignals` records the earliest blocking signal time.
- `go/internal/wave/measure.go` — passes the cycle verdict into the cost input of the wave measure.
- `go/internal/wavemeasure/cost.go` — `verdictMedians` adds the median minutes to verdict and the median fail-fast ratio to `Cost`.
- `go/cmd/evolve/cmd_wave_status.go` — `waveVerdictText` adds the per-cycle values to the cycle line.
- `go/cmd/evolve/cmd_wave_measures.go` — the cost line shows the two wave medians.

## Design Decisions
A cycle with no LLM ledger, or with no blocking signal, gets no invented fail point. The ratio is absent when the total is zero, so no division by zero occurs.

## Verification
The eight ACS predicates of cycle 2099 pass, and they run the built binary for the status output. The unit tests of the three packages pass. The ACS suite reports red=0.

## Compatibility
The JSON output of wave status adds fields only. Existing fields keep their names and meaning.

## Limitations
The fail point uses the first gate rejection or failed phase outcome. A failure with no such signal has no fail point.
