package core

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// TestWriteSignals_RendersGoalTypeAndDeliverableKind — ADR-0099: the advisor
// plans the cycle shape from the objective digest, so the digest line it reads
// must carry the goal type and the deliverable kind (the two signals that
// decide whether tdd is released and which domain phases fit).
func TestWriteSignals_RendersGoalTypeAndDeliverableKind(t *testing.T) {
	var b strings.Builder
	writeSignals(&b, router.RoutingSignals{
		Scout:  router.ScoutSignals{Present: true, CycleSizeEstimate: "medium", GoalType: "business-strategy", DeliverableKind: "document"},
		Triage: router.TriageSignals{Present: true, CycleSize: "medium", DeliverableKind: "document"},
	})
	out := b.String()
	for _, want := range []string{"goal_type=business-strategy", "deliverable_kind=document"} {
		if !strings.Contains(out, want) {
			t.Errorf("advisor digest missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "deliverable_kind=") != 2 {
		t.Errorf("both the scout and the triage line must carry deliverable_kind:\n%s", out)
	}
}
