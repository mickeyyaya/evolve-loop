# Build Explanation — Cycle 2419

## Build Binding
- Cycle: 2419
- Base SHA: cd365b81f8b78a9f330c28ec6a9047405d29e005

## Summary
Kept raised retry-ladder rungs now carry their latest defect fingerprint into `retryladder.Try`. `retryladder.BuildReport` aggregates per-rung fixed defects, and `evolve ladder report` outputs those counts in JSON and text.

## Rationale
`llmcalls.Record` and `retryladder.Try.Mark` already persist `Fingerprint`. `keepLadderRung` and `BuildReport` reuse that field instead of a new schema field, which preserves existing non-defect reports.

## Changed Areas
- `go/internal/core/retry_ladder.go` — `keepLadderRung` copies the latest entry of `st.Fingerprints` into `req.Ladder.Fingerprint` when a raised rung is kept.
- `go/internal/retryladder/report.go` — `ReportRow` adds `Defects map[string]int` (`json:"defects,omitempty"`), and `BuildReport` counts non-empty `rec.Fingerprint` values for passed non-`R0` tries.
- `go/cmd/evolve/cmd_ladder.go` — `writeLadderText` appends a `RUNG DEFECT FIXES` table sorted by rung and defect whenever any row has fixed defects.

## Design Decisions
`keepLadderRung` reuses `llmcalls.Record.Fingerprint` instead of a separate `fixed_defect` field. `ReportRow.Defects` uses `omitempty` so rows without defects keep their JSON shape, and `writeLadderText` leaves the main table unchanged.

## Verification
`go test -tags acs -count=1 ./acs/cycle2419` and `go test -count=1 ./internal/core -run TestLadderForDispatch_` pass after the final edit.

## Compatibility
Existing `llm-calls.ndjson` records without `fingerprint`, `R0` tries, and failed/incomplete tries produce no `defects` field in JSON and no defect table in text output.

## Limitations
Defect attribution in `keepLadderRung` uses the latest fingerprint in `LadderState.Fingerprints` rather than attributing multiple fingerprints separately within a single dispatch.
