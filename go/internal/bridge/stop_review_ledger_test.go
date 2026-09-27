package bridge

import (
	"bytes"
	"context"
	"testing"
	"time"
)

type stopReviewRec struct{ phase, action, reason string }

func runTmuxOnStopReview(t *testing.T, fx launchFixture, tmux *fakeTmux, rev StopReviewer,
	spy func(phase, action, reason string), extraDeps Deps, extra ...string) (int, string) {
	t.Helper()
	d := extraDeps
	d.Tmux = tmux
	d.Sleep = func(time.Duration) {}
	if d.CaptureBaseline == nil {
		// Harness default ONLY: pre-seeded artifacts stand in for mid-session
		// writes. A test whose scenario is a STALE pre-dispatch artifact must
		// pass CaptureBaseline: captureArtifactBaseline explicitly (see
		// TestRunTmuxREPL_StalePreDispatchArtifactTimesOutInsteadOfCompleting).
		d.CaptureBaseline = zeroBaselineCapture
	}
	d.Reviewer = rev
	d.OnStopReview = spy
	eng := newTestEngine(d)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

func TestRunTmuxREPL_OnStopReview_CalledForExtendAndPause(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}} // boots; artifact never lands
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "still working"},
		{Action: ReviewPause, Reason: "stalled"},
	}}
	var got []stopReviewRec
	spy := func(phase, action, reason string) {
		got = append(got, stopReviewRec{phase, action, reason})
	}
	// Tiny timeout so each loop iteration crosses a review boundary.
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, spy,
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout after pause); stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if len(got) != 2 {
		t.Fatalf("OnStopReview called %d time(s), want 2 (extend then pause); got=%+v", len(got), got)
	}
	if got[0].action != string(ReviewExtend) {
		t.Errorf("first decision action=%q, want %q", got[0].action, ReviewExtend)
	}
	if got[1].action != string(ReviewPause) {
		t.Errorf("second decision action=%q, want %q", got[1].action, ReviewPause)
	}
	if got[0].reason != "still working" || got[1].reason != "stalled" {
		t.Errorf("reasons not forwarded; got[0].reason=%q got[1].reason=%q", got[0].reason, got[1].reason)
	}
	if got[0].phase == "" || got[1].phase == "" {
		t.Errorf("phase must be forwarded (non-empty); got %+v", got)
	}
}

func TestRunTmuxREPL_OnStopReview_NilSafe(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewPause, Reason: "stalled"},
	}}
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil, // nil callback
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass")
	if code != ExitArtifactTimeout {
		t.Fatalf("nil OnStopReview must not change behavior; exit = %d, want %d; stderr=%q",
			code, ExitArtifactTimeout, stderr)
	}
}
