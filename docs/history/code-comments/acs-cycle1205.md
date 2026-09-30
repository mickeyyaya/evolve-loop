# Comment history: `acs/cycle1205`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/acs/cycle1205/predicates_test.go:3` — above `package cycle1205`

```text
// Package cycle1205 materialises the cycle-1205 acceptance criteria for this
// lane's single fleet-scoped task:
//
//   - rootcause-rule-regression-test → pin DefaultRules() against reintroducing
//     the cycle-1204 audit-REJECTED root-cause binding design.
//
// Why the task is a guard-rail, not a feature test. Cycle-1204 proposed a
// `rootCauseRule` that bound inbox items by exact string equality on a
// free-form prose field and placed it in DefaultRules() (default-on). The audit
// rejected it: D1 — measured against the 67 live .evolve/inbox items, all 20
// non-empty root_cause values were unique prose (median 317 bytes), so the rule
// was a NO-OP on real data; D2 — it carried neither of the discriminative guards
// its siblings have (hubAreaMaxItems ceiling, minAreaDepth floor), so a future
// normalising producer would collapse the campaign-less backlog into one
// over-fused cluster. The production code never landed, so there is no feature
// to regression-test; the regression worth writing is defensive — DefaultRules()
// must stay the three bounded structural signals, and none of them may bind
// items on a shared free-form prose field.
//
// Predicate strategy — every predicate below EXERCISES the system under test
// (calls DefaultRules()/Rule.Edges, or runs the package's tests as a
// subprocess); none is a source-grep of production text (the cycle-85
// degenerate-predicate ban).
//
//   - 001 (AC2) calls DefaultRules() and asserts the rule set IS exactly the
//     three structural rules AND produces zero edges for items whose only
//     commonality is an identical free-form prose field.
//   - 002 (AC4, negative) case/whitespace-varied prose must also bind nothing —
//     the "a normaliser lands upstream" failure mode of D2.
//   - 003 (AC5, edge) empty prose on every item — zero edges.
//   - 004 (AC1) runs the real package test suite and requires the NAMED
//     regression test to have actually run and PASSED (a `-run` pattern that
//     matches nothing also exits 0 — the "--- PASS:" line is what rules that
//     no-op out).
//   - 005 (AC3) the CRUX anti-no-op predicate: it MUTATES rules.go in memory
//     (go build -overlay) to reintroduce the rejected 4th prose-binding rule and
//     requires the new regression test to FAIL on that mutant. A guard that
//     cannot fail on the exact design it exists to reject is decoration. A
//     control run under the same overlay pins that the mutant still compiles,
//     so the FAIL is attributable to the guard and not to a broken build.
//
// Predicates 001-003 pin the CURRENT, audited state of production code and are
// green before Builder writes anything (recorded as pre-existing GREEN in
// test-report.md); 004 and 005 are RED until the regression test file lands at
// go/internal/inboxbatch/rules_rootcause_regression_test.go.
```

### `go/acs/cycle1205/predicates_test.go:173` — above `func TestC1205_005_RegressionTestFailsOnTheRejectedDesign(t *testing.T) {`

```text
// TestC1205_005_RegressionTestFailsOnTheRejectedDesign is AC3 and the crux
// anti-no-op predicate: the guard must be LOAD-BEARING inside
// go/internal/inboxbatch. rules.go is mutated in memory (`go test -overlay`,
// nothing written into the tree) to reintroduce the cycle-1204 rejected design —
// a 4th default-on rule binding items by exact match on a free-form prose field
// — and the regression test must FAIL on it. The control run under the same
// overlay proves the mutant compiles, so a FAIL is attributable to the guard
// rather than to a broken build.
```
