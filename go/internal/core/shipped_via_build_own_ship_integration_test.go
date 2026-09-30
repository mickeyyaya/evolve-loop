//go:build integration

package core

import (
	"context"
	"testing"
)

func TestResumeLifecycle_ShipPassPersistsTheShipLatch(t *testing.T) {
	o, st, req, rp := resumedLifecycleFixture(t) // resumes at audit → ship PASSes in the resumed session
	if _, err := o.RunCycleFromPhase(context.Background(), req, rp); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if !st.cycleState.Shipped {
		t.Errorf("resume root: ship PASS did not latch Shipped on the persisted cycle state: %+v", st.cycleState)
	}
}

func TestResumeLifecycle_PostShipSkippedVerdictReadsTheCheckpointLatch(t *testing.T) {
	cases := []struct {
		name    string
		shipped bool
		want    string
	}{
		{"shipped-before-the-pause", true, CycleOutcomeShippedViaBuild},
		{"never-shipped", false, CycleOutcomeSkippedUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, st, req, rp := resumedLifecycleFixture(t)
			rp.Phase = string(PhaseRetro)
			st.cycleState.CompletedPhases = append(st.cycleState.CompletedPhases, "audit", "ship")
			st.cycleState.FinalVerdict = VerdictSKIPPED // a post-ship floor phase's SKIPPED, persisted before the pause
			st.cycleState.Shipped = tc.shipped
			result, err := o.RunCycleFromPhase(context.Background(), req, rp)
			if err != nil {
				t.Fatalf("RunCycleFromPhase: %v", err)
			}
			if result.FinalVerdict != tc.want {
				t.Errorf("FinalVerdict = %q, want %q (checkpoint Shipped=%t)", result.FinalVerdict, tc.want, tc.shipped)
			}
		})
	}
}
