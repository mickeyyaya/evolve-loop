package bridge

// driver_tmux_repl_signalcenter_test.go — RED tests for cycle-431 slice S3
// (SignalCenter consolidation): the driver's stop-review checkpoint must
// route liveness through panestream.SignalCenter.Observe/Aggregate
// (ADR-0068), not the bare per-run detectorFor(lp) probe, and the
// reviewer's pre-S3 Progressed/Busy boolean fallback must be retired —
// verdict becomes a pure function of StopEvent.State.
//
// Seam: Deps.LivenessCenter (optional override; nil ⇒ the driver builds its
// own panestream.NewSignalCenter()) — a minimal DI seam (scout BA2) so a
// test can register a distinctive probe and prove the checkpoint actually
// consults it, without adding a new apicover-tracked symbol (Deps is
// already covered elsewhere; a struct field is not a SymbolKind apicover
// enumerates — see cmd/apicover/enumerate.go).

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

// TestRunTmuxREPL_SignalCenterStateWins (AC1, positive): a LivenessProbe
// registered on an injected SignalCenter for the "claude" profile name must
// be the state the checkpoint assigns to StopEvent.State — proof the driver
// routes through center.Observe + center.Aggregate() (ADR-0068), not a
// private per-run detectorFor(lp) probe that never learns about the
// registration. Nothing in a normal claude-tmux boot-only pane sequence
// classifies as Hung on its own, so seeing Hung here can only come from the
// registered handler winning.
func TestRunTmuxREPL_SignalCenterStateWins(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	center := panestream.NewSignalCenter()
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

// TestRunTmuxREPL_SignalCenterProbeActuallyInvoked (AC5, negative): the
// registered probe's Assess must be CALLED at least once. A driver that
// still bypasses the center (uses detectorFor(lp) directly) would never
// touch the registration at all — probe.calls stays 0 forever, which a
// state-only assertion (AC1) cannot by itself rule out.
func TestRunTmuxREPL_SignalCenterProbeActuallyInvoked(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	center := panestream.NewSignalCenter()
	probe := &fixedStateProbe{state: panestream.LivenessBusyButStagnant}
	center.RegisterHandler("claude", func() panestream.LivenessProbe { return probe })

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{}
	_, _ = runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2, LivenessCenter: center}, "--allow-bypass")

	if probe.calls == 0 {
		t.Fatal("registered SignalCenter probe was never invoked — the driver bypassed the center (AC5: center must not be bypassed)")
	}
}

// TestStopEvent_BooleanFallbackRetired (AC2, negative): the pre-S3 fallback
// that derived LivenessState from the coarse Progressed/Busy booleans when
// StopEvent.State was unset must be RETIRED (removed or provably
// unreachable), not merely shadowed. A StopEvent with State left at its
// zero value but Progressed=true/Busy=true — exactly the shape the OLD
// fallback read as Converging and extended unconditionally — must now
// PAUSE: the driver always supplies State via the center (S3), so an
// actually-unset State carries no liveness signal at all, never a
// boolean-derived extend.
func TestStopEvent_BooleanFallbackRetired(t *testing.T) {
	r := newDeterministicReviewer(2)
	ev := StopEvent{Progressed: true, Busy: true, Attempt: 0} // State left zero
	if got := r.Review(ev).Action; got != ReviewPause {
		t.Fatalf("Review(%+v).Action = %q, want pause — the Progressed/Busy boolean fallback must be retired (verdict = pure f(State))", ev, got)
	}
}

// TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant (AC6, edge): the
// cycle-291 render-wedge override — a blank pane from a LIVE session reads
// as BusyButStagnant, never Idle — must survive the migration to the
// center-authoritative State source. The scout keeps this as a post-
// Aggregate override in the driver (behavior-identical), not a center
// handler (deferred to S4).
func TestRunTmuxREPL_RenderWedgeStillPromotesToBusyStagnant(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &jiggleTmux{fakeTmux: fakeTmux{paneSeq: []string{
		tmuxPromptMarkerDefault, // boot
		"⏺ working on it…",      // post-paste baseline (non-blank)
		"",                      // every subsequent capture: blank (live session, render wedge)
	}}}
	rev := &scriptedReviewer{}
	// jiggleTmux is a *TmuxController*-satisfying wrapper over fakeTmux, so it
	// cannot go through runTmuxRev (which is typed to *fakeTmux); drive the
	// Engine directly instead, exactly as render_wedge_test.go's runTmuxWedge
	// does, but with the reviewer injected so this test can read StopEvent.State
	// rather than only the OnStopReview reason string.
	eng := NewEngine(Deps{
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
