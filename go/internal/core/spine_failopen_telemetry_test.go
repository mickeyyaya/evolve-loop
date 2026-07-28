package core

// spine_failopen_telemetry_test.go — RED contract for cycle-1166 Task 3
// (spine-failopen-telemetry, inbox weight 0.85), core half.
//
// A width-3 batch on 2026-07-13 emitted 76 occurrences of
//
//	[orchestrator] WARN spine not satisfied for next=<phase> (a mandatory
//	predecessor's handoff artifact is missing); proceeding fail-open
//	(would-block at enforce)
//
// …to stderr and nowhere else (cyclerun_select.go:151). No counter, no dossier
// field, no threshold: an epidemic with no dashboard. This item is
// measurement-first — make it visible, THEN decide whether the fix is
// artifact-retry, driver-bounce recovery, or an enforce flip.
//
// The WARN today names the phase being entered but NOT which predecessor's
// artifact is missing, so "with phase AND artifact" (the AC's words) needs the
// spine gate to REPORT its unsatisfied anchor rather than just returning false.
// That is the core-side contract below; the dossier/rollup half lives in
// internal/dossier/spine_failopen_rollup_test.go.
//
// RED today: UnsatisfiedSpineAnchor, cyclestate.SpineFailOpen and
// CycleResult.SpineFailOpens do not exist — this file does not compile.
//
// Contract Builder must satisfy:
//
//	func (sm *StateMachine) UnsatisfiedSpineAnchor(target Phase, sig router.RoutingSignals,
//	    cfg config.RoutingConfig) (Phase, bool)   // first unsatisfied predecessor anchor
//	type cyclestate.SpineFailOpen struct{ Phase, MissingArtifact, Reason string }
//	CycleResult.SpineFailOpens []SpineFailOpen
//	func (cr *cycleRun) recordSpineFailOpen(next Phase, missingArtifact, reason string)
//
// …and the fail-open branch at cyclerun_select.go:151 must call
// recordSpineFailOpen alongside the existing stderr WARN.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// spineTelemetryConfig is the mandatory-anchor spine the assertions below key
// off: build → audit → ship, the configured floor.
func spineTelemetryConfig() config.RoutingConfig {
	return config.RoutingConfig{Mandatory: []string{"build", "audit", "ship"}}
}

// TestUnsatisfiedSpineAnchor_NamesTheMissingPredecessor — the gate must be able
// to say WHICH artifact is missing, not merely that something is. Without this
// the dossier record degenerates to "(phase, unknown)" and the epidemic stays
// undiagnosable: 76 WARNs that cannot be grouped by cause.
func TestUnsatisfiedSpineAnchor_NamesTheMissingPredecessor(t *testing.T) {
	sm := NewStateMachine()
	cfg := spineTelemetryConfig()

	// No handoff artifacts at all: entering ship, the FIRST unsatisfied
	// predecessor anchor is build.
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

// TestUnsatisfiedSpineAnchor_AgreesWithSpineSatisfiedUpTo is the equivalence
// pin: the reporter and the gate must never disagree. A reporter that invents a
// missing anchor when the spine IS satisfied would inflate the very counter this
// item adds — telemetry that lies is worse than no telemetry.
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

// TestRecordSpineFailOpen_CarriesPhaseArtifactAndReason — the counter itself.
// Each fail-open must land in the cycle result with all three fields, and
// repeats must ACCUMULATE (76 occurrences must read as 76, not as 1).
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

// TestRecordSpineFailOpen_UnrecordedCycleHasNoFailOpens is the NEGATIVE twin:
// a cycle whose spine was satisfied throughout must carry ZERO records. A
// degenerate implementation that stamps a record unconditionally (or defaults
// the slice to one entry) fails here — and would make the new threshold alarm
// fire on every healthy cycle.
func TestRecordSpineFailOpen_UnrecordedCycleHasNoFailOpens(t *testing.T) {
	cr := &cycleRun{}
	if n := len(cr.result.SpineFailOpens); n != 0 {
		t.Errorf("a cycle with no fail-open events carries %d records, want 0 — a counter that "+
			"is never zero cannot detect an epidemic", n)
	}
}
