# Comment history: `acs/cycle1053`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1053/predicates_test.go:3` — above `package cycle1053`

```text
// Package cycle1053 materialises the cycle-1053 acceptance criteria for the one
// fleet-scoped item pinned to this lane: token-telemetry-s8-fleet-shadow-join
// (budgethistory median tokens/cycle + fleetbudget shadow quota join, zero
// behavior change).
//
// Predicate strategy. Every predicate DELEGATES to a real in-package Go test
// that exercises the system under test — calls budgethistory.Collect against a
// materialised run workspace, calls fleetbudget.ShadowJoin/Plan, or drives the
// composed CLI sizer quotaAwareWaveConfig — never a source-grep of production
// code (the cycle-85 degenerate-predicate ban). Each delegation requires an
// explicit "--- PASS: <Name>" marker, so a bare exit-0 from "no tests to run"
// (test renamed, deleted, or never written) is REJECTED, not silently green.
//
//   - 001/002 pin the budgethistory half: the new MedianTokensPerCycle field and
//     its absent-evidence discipline (legacy tokenless log ⇒ 0, never fabricated).
//   - 003/004 pin the fleetbudget half: the tightest-window↔tokens join and both
//     negative axes (no quota signal / no token evidence ⇒ no join).
//   - 005 is the slice's CRUX: the zero-behavior-change pin. It re-runs the
//     PRE-EXISTING Plan acceptance tests unmodified alongside the new
//     token-blindness pin, so any allocator change is a hard failure here.
//   - 006 is the WIRING proof: the join must be reachable from the composed CLI
//     wave path (quotaAwareWaveConfig), not an inert exported symbol.
```
