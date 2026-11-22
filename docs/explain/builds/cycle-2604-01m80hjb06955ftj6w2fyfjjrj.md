# Build Explanation — Cycle 2604

## Build Binding
- Cycle: 2604
- Base SHA: f34d4bc1d52cc4b4b7d5eb13d2306889ade61b33

## Summary
Added the Quality panel to `dashboard.Snapshot` and `evolve status`. It adds `--quality report|all`, stream/channel collection, and section 11 docs.

## Rationale
Exposing `Quality []QualityReading` on `dashboard.Snapshot` lets `evolve status` and the dashboard share one collector. Scanning root, cycle, and channel streams surfaces all readings.

## Changed Areas
- `go/internal/dashboard/model.go` — adds `Quality []QualityReading` (`json:"quality"`) to `Snapshot`. Pointer `Value`, `Lo`, and `Hi` fields keep zero values while omitting census bounds.
- `go/internal/dashboard/quality.go` — adds `readQuality` to collect the latest `quality.reading` per index across root, cycle, and channel streams. Absent indexes default to `DefaultStage()`.
- `go/internal/dashboard/collect.go` — populates `snap.Quality` from `readQuality(c.root)` inside `collector.collect`.
- `go/internal/dashboard/apicover_named_test.go` — names and asserts `QualityReading` on `snap.Quality` to satisfy the `.apicover-enforce` named-export contract.
- `go/internal/dashboard/quality_test.go` — adds unit tests for latest-by-TS selection, census bound omission, zero values, malformed streams, and fallback stages.
- `go/cmd/evolve/cmd_status.go` — adds `--quality report|all` (other values exit 10) and attaches `snap.Quality` to `statusReport`. Adds `writeStatusQuality` to print the `Quality` block.
- `go/cmd/evolve/cmd_status_quality_test.go` — adds unit tests covering `writeStatusQuality` formatting, stage filtering, `--quality` flag validation, and `--json` output.
- `docs/architecture/llm-quality-indexes.md` — updates section 11 to document the `--quality` flag, line format, and `quality.reading` channel source under ASD-STE100 rules.

## Design Decisions
- Kept `QualityReading.Value`, `Lo`, and `Hi` as `*float64` (`omitempty`). Valid `0.0` values serialize while census bounds and absent readings are omitted.
- Kept `buildStatusReport` signature unchanged and returned early in `writeStatusQuality` on empty slices. Existing zero-value `statusReport` unit tests stay valid without duplication.
- Rejected adding a new package because `dashboard` already owns snapshot collection and `qualitypanel` owns the index registry.

## Verification
Verified with `go test -count=1 ./internal/dashboard/...`, `go test -count=1 -run 'Quality|Status' ./cmd/evolve/...`, `go test -tags acs -count=1 ./acs/cycle2604`, and `evolve docs ste-lint --strict`.

## Compatibility
Existing `evolve status` output and `statusReport` JSON fields remain compatible. `--quality` defaults to `report` and `statusReport.Quality` uses `omitempty` when empty.

## Limitations
`evolve status` filters by each reading's `stage` (or `Index.DefaultStage()` when absent). It does not compute live readings when streams are absent.
