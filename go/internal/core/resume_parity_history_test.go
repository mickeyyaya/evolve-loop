//go:build integration

package core

import (
	"context"
	"strings"
	"testing"
)

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
