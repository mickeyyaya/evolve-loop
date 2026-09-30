# Comment history: `acs/cycle1446`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1446/predicates_test.go:3` — above `package cycle1446`

```text
// Package cycle1446 materialises the cycle-1446 acceptance criteria for the one
// fleet-scoped todo-id pinned to this lane (`context-fill-warn-threshold`).
// Both tasks close residual findings the cycle-1444 audit filed as WARN:
//
//   - contextfill-promptTokens-overflow-guard         → M1: PromptTokens sums
//     three driver-controlled counters with no overflow guard, so a wrapped sum
//     reaches FillPct and is published as a fabricated negative percentage that
//     is neither a real reading nor the documented FillPctUnmeasured sentinel.
//   - contextfill-acs-predicate-widen-run-pattern     → L3: the `-run` pattern
//     in the cycle-1444 ACS predicate and in the eval's evidence command
//     under-selects (Go `-run` is a substring match, and the word `Carry` breaks
//     `TestProductionDepsContextFill`), so the wiring-carries-through test is
//     silently skipped by both.
//
// Predicate strategy — every predicate exercises the system, never greps source
// (the cycle-85 degenerate-predicate ban):
//
//   - 001–002 call the real tokenusage API over the real overflow inputs, and
//     pin the anti-overfit half (honest large/over-full readings survive).
//   - 003 shells the task's own verifiableBy suite — ONE named package,
//     narrowed with -run, per the flaky-predicate-shape rules.
//   - 004–005 are the L3 predicates and are deliberately indirect: each READS
//     the `-run` pattern as literally written in the artifact under repair and
//     then EXECUTES it, asserting both wiring tests are selected. Asserting the
//     pattern string alone would be a grep; running the pattern the artifact
//     actually carries is the behaviour the criterion is about.
```

### `go/acs/cycle1446/predicates_test.go:173` — above `func TestC1446_005_ACSPredicateRunPatternSelectsBothWiringTests(t *testing.T) {`

```text
// TestC1446_005_ACSPredicateRunPatternSelectsBothWiringTests — L3, predicate
// half. Same defect in the cycle-1444 reachability predicate: it shells a
// pattern that matches only the out-of-range test, so the
// composition-root-carries-the-threshold half was never actually gated.
```
