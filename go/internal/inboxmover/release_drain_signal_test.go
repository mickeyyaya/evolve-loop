package inboxmover

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestApplyFailure_SystemHaltedCycleReleasesClaims(t *testing.T) {
	opts, root, got := signalledOptions(t)
	claimItem(t, root, 9, "held-item")

	res, err := ApplyCycleOutcome(opts, CycleOutcome{Cycle: 9, Passed: false, SystemLevel: true, Reason: "system halted"})
	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if len(res.Released) != 1 {
		t.Fatalf("Released = %v, want the one held claim", res.Released)
	}
	released := eventsOfKind(*got, signalcenter.KindInboxReleased)
	if len(released) != 1 || released[0].Cycle != 9 {
		t.Errorf("inbox.released events = %+v, want exactly one for cycle 9", released)
	}
}
