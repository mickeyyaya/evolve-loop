# Build Explanation — Cycle 2428

## Build Binding
- Cycle: 2428
- Base SHA: 38bc28bc639269d8e738b125443e9ce24a44a869

## Summary
Cycle verdicts and wave readings now count host minutes and host load from gate spans beside LLM minutes.

## Rationale
`cyclespend.Verdict` and the wave printers previously showed only LLM minutes, so host time in gate checks stayed invisible. Recording each gate run as a `GateSpan` in `gate-spans.ndjson` surfaces host time without altering LLM fields.

## Changed Areas
- `go/internal/cyclespend/verdict.go` — adds `HostMinutesToVerdict` and `Verdict.WithHost(spans)` to sum gate span minutes beside LLM minutes.
- `go/internal/cyclespend/gatespan.go` — defines `GateSpan`, `GateSpansFile`, `ErrGateSpans` and `ReadGateSpans` for the workspace ledger.
- `go/internal/cyclespend/sampler.go` — implements `Sampler` (`NewSampler`, `Measure`, `Spans`, `Flush`) to time gate runs and record host load.
- `go/internal/cyclespend/loadavg.go` — provides `ReadLoadAverage` and `ParseLoadAverage` as the single host load reader.
- `go/internal/wave/facts.go` — updates `readVerdict` to read `gate-spans.ndjson` into `Cycle.LLMVerdict` and report malformed lines as warnings.
- `go/internal/wavemeasure/cost.go` — adds `HostMinutesPerShippedCycle` and `MedianHostMinutesToVerdict` to `Cost` and `verdictMedians`.
- `go/cmd/evolve/cmd_wave_status.go` — updates `waveVerdictText` to print host minutes beside LLM minutes when host minutes exceed zero.
- `go/cmd/evolve/cmd_wave_measures.go` — updates `writeWaveCostText` to print host minutes per shipped cycle and median host minutes to verdict.
- `go/cmd/evolve/cmd_code_bench.go` — delegates `hostLoadAverage` and `parseLoadAverage` to `cyclespend.ReadLoadAverage` and `cyclespend.ParseLoadAverage`.
- `docs/architecture/packages/internal-cyclespend.md` — documents `WithHost`, `GateSpan`, `Sampler`, `gate-spans.ndjson` and the shared load average reader.

## Design Decisions
- Store gate spans in `gate-spans.ndjson` inside each cycle workspace so `wave.ReadCycles` reads them independently of `llm-calls.ndjson`.
- Keep `HostMinutesToVerdict` with `omitempty` on `cyclespend.Verdict`, while `Cost.HostMinutesPerShippedCycle` and `Cost.MedianHostMinutesToVerdict` serialize as `null` when undefined.
- Share `ReadLoadAverage` and `ParseLoadAverage` in `cyclespend` instead of duplicating `/proc/loadavg` and `sysctl` parsing in `cmd/evolve`.

## Verification
Unit tests across `cyclespend`, `wave`, `wavemeasure`, and `cmd/evolve` pass, and all seven ACS predicates in `go/acs/cycle2428` pass.

## Compatibility
Existing LLM-only verdicts and text outputs stay byte-equal when no gate spans exist.

## Limitations
Production gate runners (`build-floor` and `ship`) live on protected paths (`guards.IsProtectedScope`), so wiring `Sampler.Measure` and `Sampler.Flush` into those callers remains a console follow-up.
