# Build Explanation — Cycle 2239

## Build Binding
- Cycle: 2239
- Base SHA: 2937a94644401b7b97d331e9eee2eab76d112d15

## Summary
The integration batch CI watcher now polls and waits while a GitHub Actions run remains in progress instead of prematurely failing on an empty conclusion.

## Rationale
GitHub Actions run view returns an empty conclusion while a workflow run is in progress or queued. Polling with the existing settle hook allows in-progress runs to complete and land cleanly without false failures.

## Changed Areas
- `go/cmd/evolve/cmd_integration.go` — forwards the environment registration wait settle hook to the integration watch command.
- `go/internal/integration/ghci.go` — adds status inspection and settle polling loop to GhCI.verdict with ErrRunTimeout when in-progress runs exceed the deadline or try limit.

## Design Decisions
The polling loop reuses the existing GhCI.Settle hook and decodes status alongside conclusion, preserving existing refusal behavior for completed runs with empty conclusions.

## Verification
Targeted predicates in go/acs/cycle2239 and unit tests in go/internal/integration cover in-progress transitions to success, deadline timeouts, completed empty conclusion refusals, and failing runs.

## Compatibility
Existing conclusion values and refusal messages for completed runs remain identical, preserving compatibility with all integration callers.

## Limitations
Polling is bounded by the context deadline and a maximum limit of 720 tries.
