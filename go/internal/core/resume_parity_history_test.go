//go:build integration

// resume_parity_history_test.go — resume-path parity for the retro (history)
// branch. Internal package: it drives the shared resumedLifecycleFixture.
package core

import (
	"context"
	"strings"
	"testing"
)

// TestRunCycleFromPhase_HistoryBranchSurfacesDispositionGate pins the S2
// disposition gate on the resume path. The fresh history branch
// (cyclerun_record.go) runs finalizeRetroCompletion and PREPENDS its error to
// the retro reason, because "an absent/invalid disposition is surfaced loudly
// in RetroDecision, never silently recorded clean" — the cycle-1046 live gap.
// Resume's history branch mirrored the bookkeeping-regrade bound right beside
// it (with a comment calling out resume-path parity) but not this gate, so a
// resumed cycle records a clean retro decision over a disposition that was
// never verified.
func TestRunCycleFromPhase_HistoryBranchSurfacesDispositionGate(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t)
	rp.Phase = string(PhaseRetro)
	st.cycleState.CompletedPhases = append(st.cycleState.CompletedPhases, "audit")
	// No disposition artifact is written into the workspace, so the gate must
	// refuse to record the retro as clean.
	res, err := o.RunCycleFromPhase(context.Background(), req, rp)
	if err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if !strings.Contains(res.RetroDecision, "disposition-gate") {
		t.Errorf("RetroDecision = %q, want it to surface the disposition gate — an unverified disposition must never be recorded silently clean", res.RetroDecision)
	}
}
