package bridge

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
)

type fixedStateProbe struct {
	state panestream.LivenessState
	calls int
}

func (p *fixedStateProbe) Assess(_ string, _ panestream.PaneProfile) (panestream.LivenessState, float64) {
	p.calls++
	return p.state, 1
}

func TestRunTmuxREPL_SignalCenterStateWins(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	center := panestream.NewLivenessCenter()
	probe := &fixedStateProbe{state: panestream.LivenessHung}
	center.RegisterHandler(panestream.FormatClaudeREPL, func() panestream.LivenessProbe { return probe })

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
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

func TestRunTmuxREPL_SignalCenterProbeActuallyInvoked(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	center := panestream.NewLivenessCenter()
	probe := &fixedStateProbe{state: panestream.LivenessBusyButStagnant}
	center.RegisterHandler(panestream.FormatClaudeREPL, func() panestream.LivenessProbe { return probe })

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{}
	_, _ = runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2, LivenessCenter: center}, "--allow-bypass")

	if probe.calls == 0 {
		t.Fatal("registered LivenessCenter probe was never invoked — the driver bypassed the center (AC5: center must not be bypassed)")
	}
}

func TestStopEvent_BooleanFallbackRetired(t *testing.T) {
	t.Parallel()
	r := newDeterministicReviewer(2)
	ev := StopEvent{Progressed: true, Busy: true, Attempt: 0}
	if got := r.Review(ev).Action; got != ReviewPause {
		t.Fatalf("Review(%+v).Action = %q, want pause — the Progressed/Busy boolean fallback must be retired (verdict = pure f(State))", ev, got)
	}
}

func TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant(t *testing.T) {
	t.Parallel()
	fx := newFixture(t, "claude-tmux", "")
	tmux := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault,
		"⏺ working on it…",
		"",
	}}}
	rev := &scriptedReviewer{}
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
