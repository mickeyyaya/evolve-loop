package bridge

import "testing"

// TestRunTmuxREPL_TransientPaneSkipsFullArtifactTimeout reproduces the
// transient-artifact-timeout defect: an idle pane that continuously displays
// the live 529 upstream error must stop through the existing exit-81 path
// after its 60-second dwell, before the normal artifact-timeout reviewer runs.
func TestRunTmuxREPL_TransientPaneSkipsFullArtifactTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	const pane = "API Error: 529 Overloaded. This is a server-side issue, usually temporary — try again in a moment.\n❯"
	if !classifyTransientPane("claude-tmux", pane) {
		t.Fatal("529 upstream error no longer matches the claude transient_regex")
	}

	tmux := &fakeTmux{paneSeq: []string{pane}}
	reviewer := &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewPause, Reason: "full timeout reached"}}}
	code, stderr := runTmuxOnStopReview(t, fx, tmux, reviewer, nil,
		Deps{ArtifactTimeoutS: 120, RecoveryStage: "enforce"}, "--allow-bypass", "--agent=router")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%q", code, ExitArtifactTimeout, stderr)
	}
	if len(reviewer.events) != 0 {
		t.Fatalf("transient 529 pane reached the %ds artifact-timeout reviewer instead of fast-failing after its 60s dwell; reviews=%+v", 120, reviewer.events)
	}
}
