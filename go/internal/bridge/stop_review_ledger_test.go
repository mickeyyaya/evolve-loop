package bridge

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// stop_review_ledger_test.go — RED contract for cycle-188 Task 1
// (stop-review-ledger-trail), bridge side: the *-tmux REPL driver must
// surface EVERY stop-review verdict (extend AND pause) through a nil-safe
// Deps.OnStopReview(phase, action, reason) callback, so the orchestrator
// can append a kind=stop_review ledger entry (ADR-0026 Stage 1 #5).
//
// Before the Builder adds Deps.OnStopReview these tests FAIL TO COMPILE
// ("unknown field OnStopReview in struct literal") — the correct RED for a
// field-add AC in Go. Once the field exists and the driver calls it, the
// assertions below pin the behavior.

// stopReviewRec captures one OnStopReview invocation.
type stopReviewRec struct{ phase, action, reason string }

// runTmuxOnStopReview drives a claude-tmux launch with a scripted reviewer
// and an OnStopReview spy, mirroring runTmuxRev (stopreview_test.go) plus
// the new callback seam. extraDeps carries typed Deps overrides (e.g.
// ArtifactTimeoutS); use Deps{} for no overrides.
func runTmuxOnStopReview(t *testing.T, fx launchFixture, tmux *fakeTmux, rev StopReviewer,
	spy func(phase, action, reason string), extraDeps Deps, extra ...string) (int, string) {
	t.Helper()
	d := extraDeps
	d.Tmux = tmux
	d.Sleep = func(time.Duration) {}
	d.Reviewer = rev
	d.OnStopReview = spy
	eng := NewEngine(d)
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("claude-tmux", extra...), nil, &stdout, &stderr)
	return code, stderr.String()
}

// TestRunTmuxREPL_OnStopReview_CalledForExtendAndPause — AC1 + AC2: the
// driver invokes OnStopReview once per review decision, for BOTH the extend
// and the pause verdict, forwarding the phase, the action string, and the
// verdict reason. A driver that only reported the terminal (pause) decision —
// or only stderr-logged, as today — fails this.
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

// TestRunTmuxREPL_OnStopReview_NilSafe — AC2 (nil-safe): when no callback is
// wired (production default before the orchestrator opts in), the driver must
// not panic and must still exit ExitArtifactTimeout on a pause verdict.
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
