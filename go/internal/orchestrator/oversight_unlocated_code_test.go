package orchestrator

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestOversight_TheUnlockedLandingCodeIsRegisteredAsAWarn(t *testing.T) {
	f := newFixture(t, StageEnforce)
	f.addLane("A", 41)
	f.start()

	f.send("cycle", landedEvent(unlocatedCycle))

	got := f.events.withCode(CodeLandingUnlocated)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn {
		t.Fatalf("events with %s = %+v, want one WARN", CodeLandingUnlocated, got)
	}
}
