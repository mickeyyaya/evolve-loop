# Comment history: `acs/cycle518`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle518/predicates_test.go:3` — above `package cycle518`

```text
// Package cycle518 materialises the cycle-518 acceptance criteria.
//
// TRIAGE COMMITTED ONE ## top_n TASK this cycle (triage-decision.json):
//
//	carryover-todo-expiry-never-set (bug — CarryoverTodo.ExpiresAt is the field
//	the loop-start failurelog.PruneExpiredCarryoverTodos pass reads to age
//	entries out; without it state.json:carryoverTodos grows unboundedly). The
//	fix must stamp a TTL on both records the failure-learning path creates —
//	the per-failed-phase CarryoverTodo + its sibling FailedRecord — and must
//	inherit (never fabricate) that stamp for defect-derived todos.
//
// (tasks wire-fleet-width-topn-selection / immediate-binary-drift-self-repin and
// all cycle-N-failed-* stubs are DEFERRED — no predicates authored for them, per
// R9.3: predicates bind ONLY to triage-committed work.)
//
// ── PRE-EXISTING GREEN (transparently reported) ────────────────────────────
// The production fix for this exact task landed in cycle 516 (HEAD 8808db17):
// recordFailureLearning stamps record.ExpiresAt = failurelog.ComputeExpiresAt(...)
// and shares it onto the created todo (failure_learning.go:272-286), and
// ApplyDefectsAsCarryoverTodos inherits record.ExpiresAt (failure_learning.go:578).
// Triage re-committed the lingering carryover stub whose root cause already
// shipped. These predicates therefore PIN an already-satisfied contract — they
// are GREEN at TDD time. They are NOT degenerate: each drives a real in-package
// test that CALLS the creation path and asserts on the stamped field, so reverting
// the stamp (or fabricating a bogus one) turns the driven test — and thus the
// predicate — RED. The Builder has no production code to write; it must not modify
// the tests. See test-report.md "## RED Run Output" for the non-degeneracy proof.
//
// Predicate strategy (mirrors cycle507/cycle514): BEHAVIORAL predicates drive the
// system under test through its in-package tests via subprocess `go test`,
// asserting a non-degenerate pass (requireTestsRan closes the cycle-85
// "no tests to run" trap) — never a source grep. The driven tests:
//
//	internal/core/failure_learning_expiry_test.go  (creation-site stamp: positive + edge/compose)
//	internal/core/carryover_ttl_stamp_test.go      (inheritance: positive + negative/anti-fabrication)
```
