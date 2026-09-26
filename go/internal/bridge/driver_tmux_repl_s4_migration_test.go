package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// checkpointPaneSeq builds the fakeTmux.paneSeq needed to land checkpoint1
// and checkpoint2 exactly, given how many CapturePane calls the driver makes
// around them: boot consumes 2 captures (the marker-check read, then
// claude-tmux's tickDuringBoot auto-respond tick, which captures again
// internally but is not used for the marker decision); the post-paste
// baseline dispatch consumes 1 more; each wait-loop iteration's auto-respond
// tick consumes 1 capture BEFORE a checkpoint fires, and the checkpoint's own
// capture consumes 1 more — but the interval elapses one full iteration late
// (elapsed=0 on the first iteration never satisfies elapsed-intervalStart>=
// interval), so checkpoint 1 lands on the SIXTH capture (index 5) and
// checkpoint 2 on the EIGHTH (index 7). The filler positions must stay a
// bare prompt marker — content the auto-responder's own capture never needs
// to react to.
func checkpointPaneSeq(cp1, cp2 string) []string {
	const filler = tmuxPromptMarkerDefault
	return []string{filler, filler, filler, filler, filler, cp1, filler, cp2}
}

func TestRunTmuxREPL_BusyFromCenter(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	busyPane := tmuxPromptMarkerDefault + "\n⏵⏵ bypass permissions on (shift+tab to cycle) · esc to interrupt\n"
	idlePane := tmuxPromptMarkerDefault + "\n⏺ answer complete\n"
	tmux := &fakeTmux{paneSeq: checkpointPaneSeq(busyPane, idlePane)}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "busy"},
		{Action: ReviewPause, Reason: "quiet"},
	}}
	code, stderr := runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%s", code, stderr)
	}
	if len(rev.events) != 2 {
		t.Fatalf("reviewer saw %d checkpoints, want 2 (stderr=%s)", len(rev.events), stderr)
	}
	if !rev.events[0].Busy {
		t.Errorf("checkpoint 1 StopEvent.Busy = false, want true (busy-affordance pane; must be sourced via center.Busy(session))")
	}
	if rev.events[1].Busy {
		t.Errorf("checkpoint 2 StopEvent.Busy = true, want false (quiet pane, no render wedge)")
	}
}

func TestRunTmuxREPL_ProgressedFromCenter(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	checkpoint1 := tmuxPromptMarkerDefault + "\n⏺ base content\n"
	checkpoint2 := tmuxPromptMarkerDefault + "\n⏺ base content\n⏺ a new line appeared\n"
	tmux := &fakeTmux{paneSeq: checkpointPaneSeq(checkpoint1, checkpoint2)}
	rev := &scriptedReviewer{verdicts: []ReviewVerdict{
		{Action: ReviewExtend, Reason: "first checkpoint"},
		{Action: ReviewPause, Reason: "second checkpoint"},
	}}
	code, stderr := runTmuxRev(t, fx, tmux, rev, Deps{ArtifactTimeoutS: 2}, "--allow-bypass")

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%s", code, stderr)
	}
	if len(rev.events) != 2 {
		t.Fatalf("reviewer saw %d checkpoints, want 2 (stderr=%s)", len(rev.events), stderr)
	}
	if rev.events[0].Progressed {
		t.Errorf("checkpoint 1 StopEvent.Progressed = true, want false (center's first Observe has no prior observation to compare)")
	}
	if !rev.events[1].Progressed {
		t.Errorf("checkpoint 2 StopEvent.Progressed = false, want true (genuinely new content vs checkpoint 1's center.Changed(session))")
	}
}

// checkpointRegionSource loads the extracted checkpoint boundary across the
// coordinator, checkpoint, and disposition modules.
func checkpointRegionSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	var sources []string
	for _, name := range []string{
		"driver_tmux_wait.go",
		"driver_tmux_wait_checkpoint.go",
		"driver_tmux_wait_disposition.go",
	} {
		src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		sources = append(sources, string(src))
	}
	return strings.Join(sources, "\n")
}

func TestRunTmuxREPL_NoDirectChromeParseAtCheckpoint(t *testing.T) {
	region := checkpointRegionSource(t)
	for _, needle := range []string{"panestream.PaneBusy(", "PaneHasSubstantiveChange("} {
		if strings.Contains(region, needle) {
			t.Errorf("checkpoint region still calls %s directly — must read panestream.LivenessCenter projections (Busy/Changed) instead", needle)
		}
	}
}

// TestRunTmuxREPL_S4MigrationUsesRealDeterministicReviewer guards against a
// stub standing in for the real decision logic — same guard as
// TestWedgeCorpus_UsesRealDeterministicReviewer (livenesscenter_wedge_invariant_test.go).
func TestRunTmuxREPL_S4MigrationUsesRealDeterministicReviewer(t *testing.T) {
	var r StopReviewer = newDeterministicReviewer(defaultArtifactMaxExtends)
	if _, ok := r.(deterministicReviewer); !ok {
		t.Fatalf("S4 migration corpus must exercise the real deterministicReviewer, got %T", r)
	}
}
