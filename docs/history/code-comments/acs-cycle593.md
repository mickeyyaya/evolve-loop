# Comment history: `acs/cycle593`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle593/predicates_test.go:3` — above `package cycle593`

```text
// Package cycle593 materialises the cycle-593 acceptance criteria for the
// single triage-committed top_n task (triage-report.md `## top_n`; this
// cycle's fleet_scope assigns exactly one item — every other scout-selected
// candidate was routed to `## dropped` as out-of-scope-fleet, so no predicate
// binds to them here, per the AC-Materialization Contract's
// "predicates bind ONLY to triage-committed work" rule):
//
//   - token-telemetry-s1-transcript-scanner (inbox 0.95) — new leaf package
//     internal/tokenusage; RED now (package does not compile, every symbol
//     undefined). TestC593_001..004.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…574
// precedent). Each predicate shells `go test -run` over the RED unit tests
// authored this cycle in internal/tokenusage. None is a source-grep — every
// one exercises the system under test and asserts on its result.
```
