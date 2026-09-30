# Comment history: `acs/cycle637`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle637/predicates_test.go:3` — above `package cycle637`

```text
// Package cycle637 materialises the acceptance criteria for the single
// triage-committed top_n task of cycle 637, resume-dynamic-phase-transition
// (weight 0.93, inbox 2026-07-10T01-30-00Z-resume-dynamic-phase-transition.json).
//
// Defect: `evolve loop --resume` replays the checkpointed advisor-inserted phase
// (bug-reproduction / fault-localization — real catalog phases the dynamic
// router splices onto bugfix cycles); the phase re-runs and PASSes, but the
// transition kernel consulted afterwards (core.Orchestrator.RunCycleFromPhase's
// o.sm.Next call) only knows the static spine. current.IsValid()==false for the
// inserted phase, so Next returns "core: invalid phase: <phase>", the resumed
// cycle dies, and — because that error-return escapes the ADR-0044 C1 recording
// chokepoint — it is paged FAILED_UNEXPLAINED with no abort_reason. Evidence:
// 2026-07-10 resume of cycle 635 died "transition from bug-reproduction: core:
// invalid phase: bug-reproduction" / outcome FAILED_UNEXPLAINED.
//
// Predicate strategy: behavioural-via-subprocess (the cycle-549…623 precedent).
// Each predicate shells `go test -run` over one RED regression test authored
// this cycle in internal/core/resume_dynamic_transition_test.go. Every one
// EXERCISES the system under test — core.Orchestrator.RunCycleFromPhase driven
// with a real ResumePoint, a real on-disk routing-plan.json artifact, and the
// real recordPhaseOutcome sidecar — and asserts on the resulting behavior (the
// planned successor dispatches / no invalid-phase error / an abort_reason
// sidecar is written). None is a source-grep. RED now: internal/core fails
// these three tests (o.sm.Next returns invalid-phase; the failure escapes the
// chokepoint). GREEN once Builder rehydrates the transition kernel from
// routing-plan.json, degrades gracefully when it is absent, and routes every
// resume terminal path through recordPhaseOutcome.
//
// The fourth Acceptance Criteria Summary line ("go test -race on touched
// packages PASS; apicover clean") is dispositioned manual+checklist in
// test-report.md (a repo-wide toolchain gate the cycle audit already runs), not
// predicated here.
```

### `go/acs/cycle637/predicates_test.go:83` — above `func TestC637_003_TransitionFailureRecordsAbortReason(t *testing.T) {`

```text
// TestC637_003_TransitionFailureRecordsAbortReason — AC3: a resume terminal path
// that cannot continue after a transition funnels through the ADR-0044 C1
// chokepoint (recordPhaseOutcome), writing a <phase>-usage.json sidecar with a
// non-empty abort_reason — so the outcome is FAILED_EXPLAINED, never the
// FAILED_UNEXPLAINED the cycle-635 resume produced.
```
