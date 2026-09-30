# Comment history: `acs/cycle596`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle596/predicates_test.go:3` — above `package cycle596`

```text
// Package cycle596 materialises the cycle-596 acceptance criteria for the one
// triage-committed top_n task (see triage-report.md):
//
//   - token-telemetry-s2-collector-chain (inbox 0.94): a usage collector chain
//     in go/internal/tokenusage composes tiers in fidelity order
//     (transcript > eventsResult > scrollbackPeak), returns the first NON-empty
//     tier and records its source; the eventsResult tier reuses the SAME
//     *-events.ndjson result-envelope extraction as cyclecost.parseEventsLog (no
//     duplication); the scrollbackPeak tier wraps panestream.ExtractResponseTokens
//     as an output-only floor.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…594 precedent).
// Each predicate shells `go test -run` over the internal-package unit tests in
// go/internal/tokenusage/chain_test.go that materialise the criteria — every
// test exercises the real Chain / collector functions (their return value +
// recorded Source), none is a source grep. RED today: chain_test.go references
// the not-yet-implemented Chain/Collector API, so package tokenusage fails to
// compile and every `go test` below exits non-zero. Builder makes them GREEN by
// implementing the chain (do NOT modify the tests).
```
