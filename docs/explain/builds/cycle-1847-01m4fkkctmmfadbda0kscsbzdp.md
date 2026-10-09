# Build Explanation — Cycle 1847

## Build Binding
- Cycle: 1847
- Base SHA: 7d49ec1dcc8264ae085fb4a8cc4ae5cbc0b0f136

## Summary
The transcript scanner now reports a transcript line that overruns its read buffer instead of silently under-counting tokens, and the tokenusage tests lose a self-comparison, two hand-rolled helpers and stale line citations.

## Rationale
bufio.Scanner stops at an over-long line and the old loop never read its error, so a launch's usage was truncated with no signal. Surfacing the error through the existing Result.Warn keeps the scan best-effort while making the loss visible.

## Changed Areas
- `go/internal/tokenusage/scanner.go` — readLines returns the scanner error and ScanConfigRoot records it in Result.Warn.
- `go/internal/tokenusage/fillpct.go` — FillWarn merges its two early returns; the unmeasured sentinel is negative so behavior is unchanged.
- `go/internal/tokenusage/scanner_test.go` — adds the over-long line test.
- `go/internal/tokenusage/apicover_named_test.go` — compares Usage with the zero value instead of with itself.
- `go/internal/tokenusage/fallbackchain_test.go` — uses strconv.Itoa instead of a hand-rolled itoa.
- `go/internal/tokenusage/fillpct_driverwindow_test.go` — uses strconv.Itoa.
- `go/internal/tokenusage/scanner_marker_test.go` — uses json.Marshal and drops file:line citations from why-strings.

## Design Decisions
Warn rather than error: callers treat the scan as best-effort and a hard error would discard the usage that was read.

## Verification
go test -count=1 ./internal/tokenusage and the cycle1847 ACS predicates pass; the ACS suite is green.

## Compatibility
No exported signature changes; Result.Warn now may be set on a transcript hit.

## Limitations
The partial line is still skipped, so usage from the over-long turn stays uncounted; it is flagged, not recovered.
