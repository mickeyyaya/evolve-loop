package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func spineTelemetryConfig() config.RoutingConfig {
	return config.RoutingConfig{Mandatory: []string{"build", "audit", "ship"}}
}

func TestUnsatisfiedSpineAnchor_NamesTheMissingPredecessor(t *testing.T) {
	sm := NewStateMachine()
	cfg := spineTelemetryConfig()

	var empty router.RoutingSignals
	missing, ok := sm.UnsatisfiedSpineAnchor(PhaseShip, empty, cfg)
	if !ok {
		t.Fatal("UnsatisfiedSpineAnchor(ship, <no artifacts>) reported the spine satisfied — " +
			"it must agree with SpineSatisfiedUpTo, which blocks here")
	}
	if got, want := string(missing), "build"; got != want {
		t.Errorf("missing anchor = %q, want %q (the FIRST unsatisfied predecessor, so the "+
			"dossier record points at the real cause rather than the last anchor in the chain)", got, want)
	}
}

func TestUnsatisfiedSpineAnchor_AgreesWithSpineSatisfiedUpTo(t *testing.T) {
	sm := NewStateMachine()
	cfg := spineTelemetryConfig()
	var empty router.RoutingSignals

	for _, target := range []Phase{PhaseShip, PhaseBuild, Phase("scout")} {
		satisfied := sm.SpineSatisfiedUpTo(target, empty, cfg)
		_, unsatisfied := sm.UnsatisfiedSpineAnchor(target, empty, cfg)
		if satisfied == unsatisfied {
			t.Errorf("target=%s: SpineSatisfiedUpTo=%v but UnsatisfiedSpineAnchor reported "+
				"unsatisfied=%v — the reporter must be the exact complement of the gate",
				target, satisfied, unsatisfied)
		}
	}
}

func TestRecordSpineFailOpen_CarriesPhaseArtifactAndReason(t *testing.T) {
	cr := &cycleRun{}

	cr.recordSpineFailOpen(PhaseShip, "build", "would-block at enforce")
	cr.recordSpineFailOpen(Phase("audit"), "build", "digest degraded: build-report.md")

	got := cr.result.SpineFailOpens
	if len(got) != 2 {
		t.Fatalf("recorded %d spine fail-opens, want 2 — occurrences must ACCUMULATE; "+
			"collapsing repeats is how a 76-event epidemic reads as 1", len(got))
	}
	if got[0].Phase != string(PhaseShip) || got[0].MissingArtifact != "build" {
		t.Errorf("first record = %+v, want Phase=ship MissingArtifact=build", got[0])
	}
	if got[0].Reason != "would-block at enforce" {
		t.Errorf("first record Reason = %q, want the fail-open reason verbatim — the reason "+
			"string is what distinguishes a dialed-down SpineFloor from a degraded digest read", got[0].Reason)
	}
	if got[1].Phase != "audit" || got[1].Reason == got[0].Reason {
		t.Errorf("second record = %+v, want a distinct phase and its own reason", got[1])
	}
}

func TestRecordSpineFailOpen_UnrecordedCycleHasNoFailOpens(t *testing.T) {
	cr := &cycleRun{}
	if n := len(cr.result.SpineFailOpens); n != 0 {
		t.Errorf("a cycle with no fail-open events carries %d records, want 0 — a counter that "+
			"is never zero cannot detect an epidemic", n)
	}
}
