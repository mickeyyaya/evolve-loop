# Comment history: `acs/cycle1285`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1285/predicates_test.go:3` — above `package cycle1285`

```text
// Package cycle1285 holds the cycle-1285 ACS predicates for the inbox item
// `continuation-defect-ledger` (batch-integrity-review-2026-08-04.md F1).
//
// Every behavioral predicate below runs the frozen in-package contract tests as
// a SUBPROCESS and requires the named `--- PASS: <test>` receipt. Two reasons
// for that shape rather than a source assertion:
//
//   - the mechanism under test (emitDefectLedger / reconcileContinuationDefects
//     / closureClaimOffenders) is unexported and reachable only through
//     hooks.Classify, so an out-of-package predicate cannot call it directly;
//   - requiring the NAMED per-test receipt closes `go test -run` returning 0
//     on "no tests to run", which is how a deleted contract test would
//     otherwise green this suite in silence.
//
// Each invocation names ONE package and narrows with -run (never a `./...`
// sweep, never ./internal/core or ./cmd/evolve) per the flaky-predicate rules.
```
