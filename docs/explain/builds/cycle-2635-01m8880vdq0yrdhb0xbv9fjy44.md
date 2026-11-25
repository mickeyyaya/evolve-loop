# Build Explanation — Cycle 2635

## Build Binding
- Cycle: 2635
- Base SHA: e144bfba9d3fbb79d29d399556aec7b277e1c18f

## Summary
`GhCI.runIDs` now waits up to a time bound (`RegisterWithin`, default `DefaultRegisterWithin` = 5 minutes). When no CI run registers, it stops with `*NoRunError`, which wraps `ErrNoRun` and carries the bound.

## Rationale
The old `registerTries = 5` limit with a 5-second `Settle` pause stopped after about 20 seconds. A CI run that registers 2 minutes after a batch push caused `evolve integration batch` to fail with `ErrNoRun`. An elapsed-time bound on an injected clock handles late registrations and fails deterministically in tests. Exporting `NoRunError` lets external callers and tests read `Within`, `Tip`, and `Workflow` through `errors.As` without error-text assertions.

## Changed Areas
- `go/internal/integration/ghci.go` — exports `DefaultRegisterWithin`, `GhCI.Now`, `GhCI.RegisterWithin`, and `NoRunError` (`Workflow`, `Tip`, `Within`), and updates `GhCI.runIDs` to wait until `c.now().Sub(start) >= bound`.

## Design Decisions
Keeping `DefaultRegisterWithin` as a constant in `integration` with `GhCI.RegisterWithin` and `GhCI.Now` avoids edits to protected policy files. Production callers in `cmd/evolve` keep zero-value defaults (`time.Now` and `DefaultRegisterWithin`). Exporting `NoRunError` with `Unwrap() error` keeps `errors.Is(err, ErrNoRun)` intact while tests inspect `Within` and `Tip` as typed fields. Polling `run list` one more time after the final `Settle` at the bound catches a run that registered during that pause.

## Verification
`go vet`, `go test -count=1 ./internal/integration`, `go test -tags acs -count=1 ./acs/cycle2635`, `go test -count=1 -run 'Test.*Integration|TestJourney.*|TestRunBatch.*' ./cmd/evolve`, and `go test -count=1 -run TestTreeRatchet ./internal/testaudit` all pass.

## Compatibility
`GhCI` values with zero `Now` and `RegisterWithin` fields use `time.Now` and `DefaultRegisterWithin` (`5m0s`). `GhCI` with `Settle == nil` (used by `PeekCI`) still returns `*NoRunError` wrapping `ErrNoRun` after one poll without wait.

## Limitations
`DefaultRegisterWithin` is a package constant rather than a `policy.json` field; a policy override remains a follow-up in protected policy files.
