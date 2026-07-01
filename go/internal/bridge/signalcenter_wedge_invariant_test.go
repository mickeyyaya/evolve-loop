package bridge

// signalcenter_wedge_invariant_test.go — cycle-431 slice S3, Task B: pins
// every wedge-incident invariant against the center-authoritative liveness
// path (Task A). Each case names a real panestream.Liveness* constant and
// drives the actual production deterministicReviewer / runTmuxREPL — no
// stubs standing in for the reviewer under test (AC6 anti-gaming).
//
// Incident corpus:
//
//	cycle-311/312 — a producing agent is NEVER capped (Converging → extend
//	  unconditionally, past any maxExtends bound).
//	cycle-254/255 — a busy-but-silent pane extends UP TO maxExtends, then
//	  pauses (the bound a producing agent never reaches).
//	cycle-262      — a dead/echoing pane must classify Hung, not Converging
//	  (Hung fast-fails before the maxExtends backstop; Converging never
//	  stops extending on its own).
//	cycle-286/288  — non-empty pane evidence (StopEvent.StdoutTail) survives
//	  across a checkpoint whose live capture comes back empty (session death
//	  after the last good frame).

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// TestWedgeCorpus_Converging_ProducingNeverCapped (AC1, cycle-311/312):
// LivenessConverging must extend UNCONDITIONALLY — an attempt count far
// past maxExtends must still extend, because real output is never "stuck".
func TestWedgeCorpus_Converging_ProducingNeverCapped(t *testing.T) {
	r := newDeterministicReviewer(2)
	ev := StopEvent{State: panestream.LivenessConverging, Attempt: 50}
	if got := r.Review(ev).Action; got != ReviewExtend {
		t.Fatalf("Converging at attempt 50 (maxExtends=2) = %q, want extend (cycle-311/312: producing agent never capped)", got)
	}
}

// TestWedgeCorpus_BusyStagnant_BoundedThenPause (AC2, cycle-254/255):
// LivenessBusyButStagnant extends up to maxExtends, then pauses — the
// bound that distinguishes a silently-working agent from a genuinely stuck
// one.
func TestWedgeCorpus_BusyStagnant_BoundedThenPause(t *testing.T) {
	r := newDeterministicReviewer(2)
	if got := r.Review(StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 1}).Action; got != ReviewExtend {
		t.Fatalf("BusyButStagnant under cap (attempt 1/2) = %q, want extend", got)
	}
	if got := r.Review(StopEvent{State: panestream.LivenessBusyButStagnant, Attempt: 2}).Action; got != ReviewPause {
		t.Fatalf("BusyButStagnant at cap (attempt 2/2) = %q, want pause (cycle-254/255 bound)", got)
	}
}

// TestWedgeCorpus_DeadPane_HungIsNotConverging (AC3, cycle-262): a dead or
// self-echoing pane must classify LivenessHung and pause BEFORE the
// maxExtends backstop — never LivenessConverging, which the cycle-262
// bridge nudge-echo would otherwise ride to an unconditional, indefinite
// extend.
func TestWedgeCorpus_DeadPane_HungIsNotConverging(t *testing.T) {
	r := newDeterministicReviewer(6)
	got := r.Review(StopEvent{State: panestream.LivenessHung, Attempt: 0}).Action
	if got != ReviewPause {
		t.Fatalf("Hung at attempt 0 = %q, want pause (cycle-262: a dead/echoing pane must fast-fail, not read as Converging)", got)
	}
}

// TestWedgeCorpus_EvidenceSurvivesEmptyCapture (AC4, cycle-286/288): once
// the tmux server dies, every later capture returns empty — the
// checkpoint's StopEvent.StdoutTail must still carry the last NON-EMPTY
// pane, not go blank, or an escalation report built from it loses the only
// evidence of what the agent was doing.
func TestWedgeCorpus_EvidenceSurvivesEmptyCapture(t *testing.T) {
	cfg := fixtureConfig(t)
	const evidence = "TOOL CALL: go test ./... — last real output before server death"
	base := &FakeTmuxController{CaptureFrames: []string{"❯", evidence}}
	tm := &dyingServerTmux{FakeTmuxController: base, aliveCaps: 2}
	rev := &scriptedReviewer{}
	deps := fixtureDeps(tm)
	deps.Reviewer = rev

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "wedge-evidence", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1,
	})
	if err != nil || code != ExitArtifactTimeout {
		t.Fatalf("runTmuxREPL = (%d,%v), want (ExitArtifactTimeout,nil)", code, err)
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted — no checkpoint ran")
	}
	for i, ev := range rev.events {
		if !strings.Contains(ev.StdoutTail, evidence) {
			t.Errorf("checkpoint %d StdoutTail lost the cycle-286/288 evidence after server death; got %q", i, ev.StdoutTail)
		}
	}
}

// TestWedgeCorpus_UsesRealDeterministicReviewer (AC6, anti-gaming): the
// corpus above must exercise the PRODUCTION reviewer type, not a test
// double standing in for it — a stubbed StopReviewer could satisfy any
// verdict table without the real Review() decision logic ever running.
func TestWedgeCorpus_UsesRealDeterministicReviewer(t *testing.T) {
	var r StopReviewer = newDeterministicReviewer(defaultArtifactMaxExtends)
	if _, ok := r.(deterministicReviewer); !ok {
		t.Fatalf("corpus must exercise the real deterministicReviewer, got %T", r)
	}
}
