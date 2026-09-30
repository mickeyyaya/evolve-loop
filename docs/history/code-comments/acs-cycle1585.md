# Comment history: `acs/cycle1585`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1585/predicates_test.go:3` — above `package cycle1585`

```text
// Package cycle1585 materializes the cycle-1585 acceptance criteria for the sole
// committed task of this fleet lane, quota-defer-short-circuits-retro
// (scout-report.md ## Selected Tasks; triage-decision.json ## top_n). Per R9.3
// no predicate binds to the lane's deferred item (quota-reset-evidence-go-producer).
//
// The defect: the all-families-quota-exhausted abort (cyclerun_dispatch.go:264-287)
// is a DEFERRED, resumable outcome — checkpoint written, typed
// ErrAllFamiliesExhausted returned, loop exits rc=5 — yet it still calls
// cr.recordFailureLearning, whose guard (failure_learning.go:344) short-circuits
// only for fl.Failed == PhaseRetro. With no clause for the sentinel the DEFERRED
// path mutates CycleState to "retro" and runs a whole retro phase against the
// quota wall that just drained every family.
//
// AC map (1:1 with scout-report.md ## Selected Tasks "Acceptance" + "verifiableBy"):
//
//	AC1 "an all-families-quota-exhausted dispatch never calls the retro runner
//	     (counting-fake asserts 0 calls)"
//	    → C1585_001 (runs the production-path counting test in internal/core)
//	AC2 "CycleState.Phase/ActiveAgent are never set to retro for this path"
//	    → C1585_002
//	AC3 "deterministic state.FailedAt bookkeeping is still recorded" + the
//	     anti-gaming edge (multiply-wrapped sentinel matched via errors.Is)
//	    → C1585_003
//	AC4 "a genuine (non-quota) failure still dispatches retro exactly once —
//	     no regression"  → C1585_004 (NEGATIVE; pre-existing GREEN, bound so a
//	     fix that short-circuits unconditionally cannot pass this cycle)
//	AC5 "go test -count=1 ./internal/core/... exits 0 overall"
//	    → manual+checklist in test-report.md (a whole-package sweep of
//	      ./internal/core is a banned flaky predicate shape under fleet load;
//	      the CI go job and the ship gate already run it)
//	AC6 "one materialized eval with code-graded checks"  → C1585_005
//
// Adversarial axes: negative (C1585_004 — the fix must NOT swallow ordinary
// failures; C1585_005 rejects a vacuous zero-command eval), edge (C1585_003 —
// the sentinel arrives multiply %w-wrapped, so an `==` identity check fails
// here), semantic (zero-dispatch, resumable cycle-state, bookkeeping survival
// and eval rigor are four distinct behaviors, not one restated).
//
// No source-grep predicates (cycle-85 rule): C1585_001..004 execute the real
// production dispatch chain as a subprocess and count named PASS markers (a
// bare exit 0 would hide a renamed or skipped test); C1585_005 runs the SSOT
// eval-quality checker. Every `go test` invocation names ONE package and is
// narrowed with -run (flaky-predicate-shape rule: no `/...` sweeps, no
// unnarrowed ./internal/core).
```

### `go/acs/cycle1585/predicates_test.go:79` — above `func TestC1585_001_all_families_exhausted_dispatches_no_retro(t *testing.T) {`

```text
// AC1: the production RunCycle chain drives scout to exit=85 on every attempt
// and the counting fake must observe ZERO retro-runner calls. A green unit test
// on the guard helper alone already passed once in cycle-1582 while the wiring
// was still broken, so the predicate binds the whole dispatch chain.
```
