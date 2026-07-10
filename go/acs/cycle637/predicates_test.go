//go:build acs

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
package cycle637

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

// runGoTest shells `go test -run '^(<pattern>)$' -count=1 <pkg>` and reports
// whether it exited cleanly plus the combined output. -count=1 defeats the test
// cache so the predicate always exercises current source. code<0 is a genuine
// launch failure (binary missing / killed by signal), never a test verdict —
// that fails loudly rather than being misread as a RED behavioral result.
func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

// TestC637_001_InsertedPhaseTransitionsViaPlan — AC1: resuming from an
// advisor-inserted phase rehydrates the transition kernel from the run's
// routing-plan.json and transitions to the next planned phase (behavioral proof:
// the planned successor's runner is dispatched), instead of dying with "core:
// invalid phase".
func TestC637_001_InsertedPhaseTransitionsViaPlan(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestRunCycleFromPhase_InsertedPhaseTransitionsViaPlan")
	if !ok {
		t.Errorf("resume does not rehydrate the transition kernel from routing-plan.json:\n%s", out)
	}
}

// TestC637_002_MissingPlanDegradesNotInvalidPhase — AC2 (degrade/negative axis):
// with routing-plan.json absent, resume degrades the inserted phase to its
// archetype and routes to the next static phase without surfacing an
// invalid-phase error.
func TestC637_002_MissingPlanDegradesNotInvalidPhase(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestRunCycleFromPhase_MissingPlanDegradesNotInvalidPhase")
	if !ok {
		t.Errorf("resume does not degrade gracefully when routing-plan.json is missing:\n%s", out)
	}
}

// TestC637_003_TransitionFailureRecordsAbortReason — AC3: a resume terminal path
// that cannot continue after a transition funnels through the ADR-0044 C1
// chokepoint (recordPhaseOutcome), writing a <phase>-usage.json sidecar with a
// non-empty abort_reason — so the outcome is FAILED_EXPLAINED, never the
// FAILED_UNEXPLAINED the cycle-635 resume produced.
func TestC637_003_TransitionFailureRecordsAbortReason(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestRunCycleFromPhase_TransitionFailureRecordsAbortReason")
	if !ok {
		t.Errorf("resume transition failure escapes the C1 chokepoint (no abort_reason recorded):\n%s", out)
	}
}
