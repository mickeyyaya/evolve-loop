package bridge

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

// fixedStateProbe is a LivenessProbe test double that always reports the
// same state and records every Assess call — the only way a test can prove
// the driver actually invoked it (vs. a bypassed center).
type fixedStateProbe struct {
	state panestream.LivenessState
	calls int
}

func (p *fixedStateProbe) Assess(_ string, _ panestream.PaneProfile) (panestream.LivenessState, float64) {
	p.calls++
	return p.state, 1
}

// A normal claude-tmux boot-only pane sequence never classifies as Hung on
// its own, so seeing Hung here can only come from the registered handler.
func TestRunTmuxREPL_SignalCenterStateWins(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	center := panestream.NewLivenessCenter()
	probe := &fixedStateProbe{state: panestream.LivenessHung}
	center.RegisterHandler("claude", func() panestream.LivenessProbe { return probe })

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never appears
	rev := &scriptedReviewer{}
	code, _ := runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2, LivenessCenter: center}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout", code)
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted — no checkpoint ran")
	}
	if rev.events[0].State != panestream.LivenessHung {
		t.Fatalf("StopEvent.State = %v, want LivenessHung (the center-registered probe must win — AC1: driver calls Observe+Aggregate)", rev.events[0].State)
	}
}

// A state-only assertion (like SignalCenterStateWins) cannot rule out a
// driver that never calls the registered probe at all; this checks the call
// directly.
func TestRunTmuxREPL_SignalCenterProbeActuallyInvoked(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	center := panestream.NewLivenessCenter()
	probe := &fixedStateProbe{state: panestream.LivenessBusyButStagnant}
	center.RegisterHandler("claude", func() panestream.LivenessProbe { return probe })

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{}
	_, _ = runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2, LivenessCenter: center}, "--allow-bypass")

	if probe.calls == 0 {
		t.Fatal("registered LivenessCenter probe was never invoked — the driver bypassed the center (AC5: center must not be bypassed)")
	}
}

func TestStopEvent_BooleanFallbackRetired(t *testing.T) {
	r := newDeterministicReviewer(2)
	ev := StopEvent{Progressed: true, Busy: true, Attempt: 0} // State left zero
	if got := r.Review(ev).Action; got != ReviewPause {
		t.Fatalf("Review(%+v).Action = %q, want pause — the Progressed/Busy boolean fallback must be retired (verdict = pure f(State))", ev, got)
	}
}

func TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault, // boot
		"⏺ working on it…",      // post-paste baseline (non-blank)
		"",                      // every subsequent capture: blank (live session, render wedge)
	}}}
	rev := &scriptedReviewer{}
	// jiggleTmux is a *TmuxController*-satisfying wrapper over fakeTmux, so it
	// cannot go through runTmuxRev (typed to *fakeTmux); drive the Engine
	// directly instead, with the reviewer injected so this test can read
	// StopEvent.State rather than only the OnStopReview reason string.
	eng := newTestEngine(Deps{
		Tmux:             tmux,
		Sleep:            func(time.Duration) {},
		ArtifactTimeoutS: 2,
		Reviewer:         rev,
	})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%s", code, stderr.String())
	}
	if len(rev.events) == 0 {
		t.Fatal("reviewer never consulted")
	}
	if rev.events[0].State != panestream.LivenessBusyButStagnant {
		t.Fatalf("StopEvent.State = %v at the first blank-live checkpoint, want LivenessBusyButStagnant (cycle-291 render-wedge override lost)", rev.events[0].State)
	}
}
