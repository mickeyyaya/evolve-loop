# Build Explanation — Cycle 2615

## Build Binding
- Cycle: 2615
- Base SHA: 2c8fc5fe94f930b3457ee2f38213adc5bc23897f

## Summary
`GhCI.runIDs` now waits up to a configurable time bound (`RegisterWithin`, default `DefaultRegisterWithin` = 5 minutes). When no CI run registers, it stops with `ErrNoRun` naming the bound.

## Rationale
The fixed `registerTries = 5` limit with a 5-second `Settle` pause ended after about 20 seconds. A CI run that registers 2 minutes after a batch push caused `evolve integration batch` to fail with `ErrNoRun`. Replacing the poll count with an elapsed-time bound measured on an injectable clock covers late registrations and fails deterministically in tests.

## Changed Areas
- `go/internal/integration/ghci.go` — adds `DefaultRegisterWithin`, `GhCI.Now`, and `GhCI.RegisterWithin`; updates `GhCI.runIDs` to loop until `c.now().Sub(start) >= bound` and return an error wrapping `ErrNoRun` that names the bound.

## Design Decisions
Keeping `DefaultRegisterWithin` as a constant in `integration` with `GhCI.RegisterWithin` and `GhCI.Now` fields avoids editing protected `policy` surfaces. Production callers (`ports()` and `watchIntegrationTip`) keep zero-value defaults. Using an injected `Now` function alongside `Settle` keeps the loop deterministic without real sleeps in unit and ACS tests. Polling `run list` once more after the final `Settle` when the bound is reached avoids dropping a run that registers during the last pause.

## Verification
`go test -count=1 ./internal/integration`, `go test -tags acs -count=1 ./acs/cycle2615 ./acs/cycle2239`, and `go test -count=1 -run 'Test.*Integration|TestJourney.*|TestRunBatch.*' ./cmd/evolve` all pass.

## Compatibility
`GhCI` values with zero `Now` and `RegisterWithin` fields default to `time.Now` and `DefaultRegisterWithin` (`5m0s`). `GhCI` with `Settle == nil` (used by `PeekCI`) still returns `ErrNoRun` after one poll without waiting.

## Limitations
`DefaultRegisterWithin` is a package constant rather than a `policy.json` field; adding a policy override remains a follow-up in protected policy files.
