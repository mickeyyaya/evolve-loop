# Comment history: `acs/cycle1062`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1062/predicates_test.go:3` — above `package cycle1062`

```text
// Package cycle1062 materialises the cycle-1062 acceptance criteria for the
// single fleet-scoped task pinned to this lane:
//
//	chronicle-s6-escalation-boundary
//	  → superseded_by: failure-disposition-router (S4 boundary applier)
//
// Because the parent design states "S4 MUST NOT land before S3's staging
// exists", scout materialised the committed task as two dependency-ordered
// halves, both of which these predicates gate:
//
//	Task 1  disposition-router-s3-floors-and-staging   → go/internal/dispositionrouter
//	Task 2  disposition-router-s4-boundary-applier     → go/internal/recurrence/apply.go
//	                                                     + go/cmd/evolve/cmd_loop.go call site
//
// Predicate strategy — every predicate here EXERCISES the system under test
// (calls the production function against a t.TempDir() fixture and asserts on
// its return value or its real on-disk side effect), never a source-grep of
// production code (the cycle-85 degenerate-predicate ban). Predicates 001-004
// drive `dispositionrouter` directly; 005-009 drive `recurrence.ApplyBoundary` directly;
// 010 shells the loop-boundary wiring test so the cmd_loop call site is proven
// by execution, not by a magic string.
//
// RED shape this cycle: the `dispositionrouter` package and `recurrence.ApplyBoundary` do
// not exist yet, so the package fails to COMPILE — the correct RED for a
// not-yet-built API (go/acs/README.md: a predicate package that fails to
// compile is a hard suite error, never a silent PASS).
//
// Isolation: no predicate reads or writes the live repo tree. Every inbox,
// escalations-staging and report path is rooted in t.TempDir(), per the cycle
// goal constraint that tests must never mutate the live tree.
```
