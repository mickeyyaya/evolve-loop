# Comment history: `acs/cycle1190`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1190/predicates_test.go:3` — above `package cycle1190`

```text
// Package cycle1190 materialises the cycle-1190 acceptance criteria for the
// three triage-committed (## top_n) tasks:
//
//	inboxbatch-class-field-capture       — Item.Class captured from JSON "class"
//	inboxbatch-operator-state-detection  — IsOperatorState pure classifier
//	evalgate-monotonic-binary-target-lint — LintMonotonicBinaryTarget lint
//
// These are the scoped prerequisite slices of the three fleet-assigned inbox
// items; the full landing mechanisms are ## deferred and therefore carry ZERO
// predicates here (R9.3 floor-binding: predicates bind only to committed work).
//
// Predicate strategy — every predicate CALLS the system under test and asserts
// on its return value; none greps production source for a magic string (the
// cycle-85 degenerate-predicate ban):
//
//   - 001 runs the real inboxbatch.LoadDir over a temp inbox holding a verbatim
//     copy of a live item's JSON and asserts Class round-trips (plus the
//     absent-key zero value), so a field added without the `json:"class"` tag
//     still fails.
//   - 002/003 call IsOperatorState directly: 002 is the positive, 003 is the
//     NEGATIVE (source-touching path, empty file list, wrong class) — a
//     `return true` stub passes 002 and fails 003.
//   - 004/005 call LintMonotonicBinaryTarget: 004 asserts it fires with a
//     direction+floor-oriented message on the exact cycle-992 AC shape, 005 is
//     the NEGATIVE (compliant delta-floor phrasing, non-monotonic class, empty
//     input) — a `return []string{msg}` stub passes 004 and fails 005.
//   - 006 EXECUTES both touched packages' suites as a subprocess and asserts
//     exit 0 (regression pin: the new field/functions must not break callers).
//
// RED state at TDD time: this package does not compile — Item.Class,
// inboxbatch.IsOperatorState and evalgate.LintMonotonicBinaryTarget do not
// exist yet. That compile failure IS the RED evidence for 001-005; 006 is the
// only predicate that would pass on today's tree once compilation succeeds.
```

### `go/acs/cycle1190/predicates_test.go:225` — above `const monotonicBinaryAC = "prune the inbox backlog to <=25 items"`

```text
// monotonicBinaryAC is the literal cycle-992 acceptance-criterion shape the lint
// exists to flag: a binary ABSOLUTE target on a monotonic convergence task.
```
