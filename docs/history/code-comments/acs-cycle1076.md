# Comment history: `acs/cycle1076`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1076/predicates_test.go:3` — above `package cycle1076`

```text
// Package cycle1076 materialises the cycle-1076 acceptance criteria for the one
// fleet-scoped task pinned to this lane (inbox item `tdd-topn-binding-gate`,
// acceptance criterion 2):
//
//   - build-selfcheck-removal-claim-check → a build-report.md claiming a file
//     removal that did NOT happen must fail build-selfcheck deterministically.
//     Part 1 of the inbox item (topngate's triage→TDD scope binding) is already
//     shipped; predicate 004 pins it as a no-regression guard.
//
// Predicate strategy — every predicate EXERCISES the system under test (the
// cycle-85 degenerate-predicate ban): 001/002/003 drive the real
// core.RemovalClaimFailures / core.DefaultBuildFloorChecks / the real
// buildFloorReviewer in-process against temp-dir fixtures; 004 shells the
// topngate package's behavioural unit tests. No predicate asserts on source text.
```

### `go/acs/cycle1076/predicates_test.go:63` — above `func TestC1076_001_FalseRemovalClaimIsDetected(t *testing.T) {`

```text
// TestC1076_001_FalseRemovalClaimIsDetected is the crux: the cycle-660 shape —
// a report claiming a scaffold was "already removed" while the file is still in
// the worktree — must yield a failure naming that path. The honest-removal half
// pins the other direction (no false blocking).
```
