package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunTmuxREPL_PauseWritesEscalationReport(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	defer os.RemoveAll(fx.ws)
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewPause, Reason: "stalled-reason"},
	}}
	var got []stopReviewRec
	spy := func(phase, action, reason string) {
		got = append(got, stopReviewRec{phase, action, reason})
	}
	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, spy,
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass", "--agent=scout")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout after pause); stderr=%q", code, ExitArtifactTimeout, stderr)
	}

	// Verify escalation-report.json exists in workspace
	reportPath := filepath.Join(fx.ws, "scout-escalation-report.json")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("escalation report %s does not exist: %v", reportPath, err)
	}

	// Read and parse report
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("failed to read escalation report: %v", err)
	}

	var report escalationReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("failed to unmarshal escalation report: %v", err)
	}

	if report.Phase != "scout" {
		t.Errorf("got phase %q, want %q", report.Phase, "scout")
	}
	if report.Action != "pause" {
		t.Errorf("got action %q, want %q", report.Action, "pause")
	}
	if report.Reason != "stalled-reason" {
		t.Errorf("got reason %q, want %q", report.Reason, "stalled-reason")
	}
	if report.StopKind != "artifact_timeout" {
		t.Errorf("got stop_kind %q, want %q", report.StopKind, "artifact_timeout")
	}
}

func TestRunTmuxREPL_ExtendNoEscalationReport(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	defer os.RemoveAll(fx.ws)

	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}

	rev := &writingReviewer{artifact: fx.artifact}

	code, stderr := runTmuxOnStopReview(t, fx, tmux, rev, nil,
		Deps{ArtifactTimeoutS: 2}, "--allow-bypass", "--agent=scout")

	if code != 0 {
		t.Fatalf("exit = %d, want 0 (success); stderr=%q", code, stderr)
	}

	// Verify escalation-report.json does NOT exist in workspace
	reportPath := filepath.Join(fx.ws, "scout-escalation-report.json")
	if _, err := os.Stat(reportPath); err == nil {
		t.Fatalf("escalation report %s should not exist for successful (completed) run", reportPath)
	}
}

// writingReviewer models the agent finishing its deliverable mid-wait: the
// artifact is written ONCE, at the first review checkpoint, and then left alone.
//
// The write-once guard is load-bearing under the cycle-1233 cross-poll
// stability window (completion.go): a reviewer that rewrote the file at every
// checkpoint would bump its mtime on every tick, so the window could never
// close and this fixture would spin forever. A real agent writes its deliverable
// and stops — the fixture now models that instead of a permanent rewriter.
type writingReviewer struct {
	artifact string
	wrote    bool
}

func (w *writingReviewer) Review(ev StopEvent) ReviewVerdict {
	if !w.wrote {
		_ = os.WriteFile(w.artifact, []byte("PROTOTYPE OK\n"), 0o644)
		w.wrote = true
	}
	return ReviewVerdict{Action: ReviewExtend, Reason: "still working"}
}
