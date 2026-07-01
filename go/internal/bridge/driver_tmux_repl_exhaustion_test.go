package bridge

import "testing"

// driver_tmux_repl_exhaustion_test.go — the wait-loop fast-fail (S2). A CLI that
// hits its quota/rate-limit MID-PHASE renders the manifest's exhausted_regex
// wall and parks at its prompt without exiting. The SignalCenter's ExhaustionProbe
// now reports LivenessExhausted for that pane; the wait loop must return
// ExitUnknownPrompt (85) so the dispatch chain fails over — NOT burn the full
// artifact timeout (81) while nudging a walled CLI (the agy hang-forever livelock).

// A pane matching the CLI's exhausted_regex fast-fails to ExitUnknownPrompt.
func TestTmuxREPL_ExhaustionWall_FailsOverFast(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Boots (prompt marker ❯ present) AND shows the quota wall (matches
	// claude-tmux's usage.exhausted_regex "reached your usage limit"). The
	// artifact is never written, so only the exhaustion override can end the run.
	walled := "❯\n⚠ You've reached your usage limit — upgrade to continue.\n❯"
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{walled}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitUnknownPrompt {
		t.Fatalf("exit = %d, want %d (ExitUnknownPrompt — exhaustion fast-fail, not %d ExitArtifactTimeout); stderr=%q",
			code, ExitUnknownPrompt, ExitArtifactTimeout, stderr)
	}
}

// Negative pin: a healthy idle pane (no wall) still concludes with the normal
// ExitArtifactTimeout — the exhaustion fast-fail must NOT fire on a pane that
// merely lacks the artifact.
func TestTmuxREPL_HealthyIdle_NotExhausted(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	code, _ := runTmuxNudge(t, fx, tmux)
	if code != ExitArtifactTimeout {
		t.Fatalf("healthy idle exit = %d, want %d (ExitArtifactTimeout) — exhaustion must not fire without a wall", code, ExitArtifactTimeout)
	}
}
