# Comment history: `acs/cycle1113`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1113/predicates_test.go:3` — above `package cycle1113`

```text
// Package cycle1113 materializes the cycle-1113 acceptance criteria for the
// sole committed item of this fleet lane, tdd-topn-binding-gate
// (triage-report.md ## top_n; fleet_scope pins this lane to that one todo-id,
// so per R9.3 no predicate here binds to a deferred or other-lane item).
//
// Task nature: COVERAGE GAP. internal/topngate is fully implemented and wired
// (cmd_cycle.go -> NewReviewer(cfg.TopNGate), default StageEnforce), and its
// unit suite is green. What does not exist is proof that tddScopeGate's ONE
// fatal path (gate.go:117-120, empty committed ## top_n + a non-empty authored
// set) survives the COMPOSITION: NewReviewer(stage).Review(Phase: PhaseTDD) —
// the exact code path `evolve loop` executes. Every reviewer_test.go case
// today drives PhaseBuild or PhaseAudit, so an appliesTo typo, a dispatch
// reordering, or a stage-comparison inversion could silently disarm the TDD
// gate with the whole suite still green.
//
// AC map (1:1, from scout-report.md ## Acceptance Criteria Summary + both
// Selected Tasks' verifiableBy):
//
//	AC1 "StageEnforce + PhaseTDD + empty top_n + authored files => Approve
//	     false, Reason non-empty" (Task 1 verifiableBy)
//	    -> C1113_001 exercises the composed reviewer directly (production
//	       behaviour today: PRE-EXISTING GREEN, bound so a regression in
//	       reviewer.go's dispatch is caught by audit regardless of what the
//	       package's own tests do) and C1113_003 (the named reviewer-level
//	       test must exist and PASS: RED until Builder writes it).
//	AC2 "StageShadow on the identical fixture => Approve true" (Task 2)
//	    -> C1113_002 (direct, PRE-EXISTING GREEN) + C1113_003 (named test).
//	AC3 "the new tests are load-bearing, not tautological — they fail on a
//	     deliberately reverted reviewer.go"
//	    -> C1113_004 (mutation: tddScopeGate dropped from the gates slice =>
//	       the enforce test MUST fail) and C1113_005 (mutation: the
//	       stage-comparison guard removed so shadow blocks too => the shadow
//	       test MUST fail). Both RED today (no such tests to kill).
//	AC4 "go test ./internal/topngate/... remains green (no regression)"
//	    -> C1113_006 counts an explicit verbose PASS for all 15 pre-existing
//	       test funcs; a bare exit 0 would hide a deleted or renamed one.
//	AC5 "no production behaviour change (gate.go/reviewer.go untouched)"
//	    -> manual+checklist in test-report.md (a diff-shape judgement, not a
//	       behavioural assertion; C1113_001/002/006 pin the behaviour that a
//	       production edit would have to preserve anyway).
//
// Adversarial axes: NEGATIVE — C1113_002 asserts the gate does NOT block at
// shadow, and C1113_004/005 assert the new tests DO fail under mutation (the
// anti-no-op signal: a test asserting `true` passes C1113_003 and dies here).
// EDGE — C1113_003 rejects the "no tests to run" exit-0 hole; C1113_006
// rejects exit-0-with-a-test-deleted. SEMANTIC — enforce blocking, shadow
// approving, mutation sensitivity and suite health are four distinct
// behaviours, not one restated.
//
// No source-grep predicates (cycle-85 rule): every predicate below either
// calls the system under test in-process (001/002) or runs it as a subprocess
// and asserts on real emitted output (003/004/005/006).
```
