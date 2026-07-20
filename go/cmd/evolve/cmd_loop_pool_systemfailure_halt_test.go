package main

// cmd_loop_pool_systemfailure_halt_test.go — RED contract for cycle-959's fleet
// lane task adr0072-fleet-pool-halt-unwired (scout-report.md sole selected task;
// inbox adr0072-fleet-halt-unwired, weight 0.97).
//
// PROBLEM (scout Key Findings, verified against the CURRENT tree): the WAVE
// dispatch branch of runLoop (cmd_loop.go:533) already calls
// anyLaneHaltedForSystemFailure(results) after dispatch and STOPS the batch on a
// forged verdict (ADR-0072). The POOL dispatch branch (cmd_loop.go:490-498, the
// opt-in policy.fleet.scheduling=="pool" path) gets back the IDENTICAL
// []fleet.Result shape (same ExitCode field, same systemFailureHaltExitCode
// contract) but NEVER consults it — it only logs the ok/failed lane count and
// `continue`s. A lane that forges a verdict under pool scheduling still files the
// escalation dossier + P0 (subprocess side), but the BATCH DOES NOT STOP — the
// exact churn-on-forged-verdict failure mode ADR-0072 exists to prevent, now
// confined to the pool code path.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to COMPILE today; that compile failure IS the
// RED evidence, mirroring the cycle-465/507/547/550/951 precedent):
//
//	// dispatchHaltDecision inspects a completed dispatch iteration's lane
//	// results and returns the ADR-0072 halt outcome BOTH the wave and pool
//	// branches apply, so the two structurally-similar branches cannot drift on
//	// the halt floor (per [[never_duplicate_centralize_via_design_patterns]]):
//	// when any lane exited with the system-failure halt code (a forged verdict)
//	// it returns halt=true with rc=systemFailureHaltExitCode and
//	// stopReason="system_failure_halt" so the caller STOPS the batch; otherwise
//	// halt=false (rc=0, stopReason="") so the caller keeps the never-stop retry
//	// semantics ordinary lane FAILs are entitled to. It MUST single-source the
//	// detection through the existing anyLaneHaltedForSystemFailure helper (AC4 —
//	// no branch re-implements the ExitCode==systemFailureHaltExitCode scan).
//	func dispatchHaltDecision(results []fleet.Result) (rc int, stopReason string, halt bool)
//
// The pool branch (cmd_loop.go `case ran:` at ~490) must then, after logging the
// lane count, apply this decision: `if rc, sr, halt := dispatchHaltDecision(results);
// halt { lr.StopReason = sr; lr.emitFatal(...); return rc }` BEFORE its `continue`
// — that inline branch glue is verified by the Auditor diff-scope checklist
// (test-report.md), exactly as the wave branch's own inline glue is; these unit
// tests pin the shared DECISION the branch consumes.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : TestDispatchHaltDecision_HaltsOnSystemFailureLane
//     (a lane with the halt code STOPS the batch — the core wiring proof).
//   - Negative  : TestDispatchHaltDecision_OrdinaryFailuresContinue
//     (strongest anti-no-op: ordinary FAIL/launch-error lanes must NOT halt —
//     a decision that halts on any non-zero exit fails here and would freeze the
//     never-stop retry loop ADR-0072 deliberately preserves).
//   - Edge/OOD  : TestDispatchHaltDecision_EmptyResultsContinue
//     (nil / empty results never halt).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

// TestDispatchHaltDecision_HaltsOnSystemFailureLane (AC1, positive): a pool
// iteration whose results include ANY lane that exited with the ADR-0072
// system-failure halt code must resolve to halt=true, rc=systemFailureHaltExitCode
// and stopReason="system_failure_halt" — the batch stops instead of dispatching
// the next pool iteration.
func TestDispatchHaltDecision_HaltsOnSystemFailureLane(t *testing.T) {
	results := []fleet.Result{
		{Index: 0, ExitCode: 0},
		{Index: 1, ExitCode: systemFailureHaltExitCode},
		{Index: 2, ExitCode: 2}, // an ordinary FAIL alongside the halt lane
	}
	rc, stopReason, halt := dispatchHaltDecision(results)
	if !halt {
		t.Fatalf("dispatchHaltDecision(halt-code lane present) halt=false, want true — the pool batch must stop on a forged verdict")
	}
	if rc != systemFailureHaltExitCode {
		t.Errorf("rc = %d, want %d (systemFailureHaltExitCode)", rc, systemFailureHaltExitCode)
	}
	if stopReason != "system_failure_halt" {
		t.Errorf("stopReason = %q, want %q", stopReason, "system_failure_halt")
	}
}

// TestDispatchHaltDecision_OrdinaryFailuresContinue (AC2, NEGATIVE / regression):
// a pool iteration with only ordinary lane failures (rc=2 FAIL, rc=1, and a
// launch error rc=-1) must resolve to halt=false, rc=0, stopReason="" — ordinary
// task-level failures keep the never-stop retry semantics ADR-0072 draws the line
// at. A decision that halts on any non-zero exit code fails here.
func TestDispatchHaltDecision_OrdinaryFailuresContinue(t *testing.T) {
	results := []fleet.Result{
		{Index: 0, ExitCode: 2},
		{Index: 1, ExitCode: 1},
		{Index: 2, ExitCode: -1, Err: errTestLaneFailed},
		{Index: 3, ExitCode: 0},
	}
	rc, stopReason, halt := dispatchHaltDecision(results)
	if halt {
		t.Fatalf("dispatchHaltDecision(ordinary failures only) halt=true, want false — ordinary lane failures must NOT stop the batch")
	}
	if rc != 0 {
		t.Errorf("rc = %d, want 0 (batch continues)", rc)
	}
	if stopReason != "" {
		t.Errorf("stopReason = %q, want empty (batch continues)", stopReason)
	}
}

// TestDispatchHaltDecision_EmptyResultsContinue (AC2, EDGE): nil and empty result
// slices never halt — the boundary case a rolling pool hits when an iteration
// realized zero completed lanes.
func TestDispatchHaltDecision_EmptyResultsContinue(t *testing.T) {
	for _, results := range [][]fleet.Result{nil, {}} {
		rc, stopReason, halt := dispatchHaltDecision(results)
		if halt || rc != 0 || stopReason != "" {
			t.Errorf("dispatchHaltDecision(%v) = (rc=%d, stop=%q, halt=%v), want (0, \"\", false)",
				results, rc, stopReason, halt)
		}
	}
}
