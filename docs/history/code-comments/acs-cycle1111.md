# Comment history: `acs/cycle1111`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1111/predicates_test.go:3` — above `package cycle1111`

```text
// Package cycle1111 materialises the cycle-1111 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	tdd-file-scope-binding-check → extend tddScopeGate with a second,
//	independent, ADVISORY file-scope overlap check.
//
// The gap. tddScopeGate today compares only PROSE labels: test-report.md's
// "## Task: <slug>" against triage-report.md's "## top_n" slugs. Both drift
// cases are advisory (cycles 916 + 1012 false rejections), and the gate's own
// comments (gate.go:73-75, 132-135) name the missing complement as "the queued
// construction-level check" — the committed item's DECLARED file scope
// (scout-report.md's "**targetFiles:**" line) vs what TDD actually authored.
// A deliverable can carry exactly the right label and touch nothing the
// committed item names; today that passes silently.
//
// Predicate strategy — every predicate drives the SHIPPING code path
// (topngate.NewReviewer(...).Review, the same constructor cmd_cycle.go wires)
// against a synthetic workspace, and asserts on the reviewer's real outputs:
// the ReviewResult and the structured logf seam it writes to os.Stderr. No
// predicate greps production source, so adding a magic string cannot green
// them (the cycle-85 degenerate-predicate ban).
//
//   - 001 is the crux: right label, disjoint file scope → the advisory MUST be
//     emitted and the review MUST still approve at enforce.
//   - 002 is the anti-false-positive: the normal shape (Scout names the
//     production file, TDD authors the sibling _test.go) must stay SILENT —
//     an advisory that fires every healthy cycle is noise, not a signal.
//   - 003 is the fail-open case: no scout-report.md → no declared scope → no
//     new signal, per this gate family's ambiguity-favours-pass convention.
//   - 004 is the anti-overcorrection guard: the one FATAL case (authoring
//     under an empty ## top_n) must still abort at enforce.
```
