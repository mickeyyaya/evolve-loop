package bridge

// driver_tmux_repl_s4_migration_test.go — RED tests for cycle-432 slice S4,
// Task 2 (s4-migrate-driver-callsites): the stop-review checkpoint must stop
// parsing CLI chrome directly (panestream.PaneBusy /
// PaneHasSubstantiveChange at driver_tmux_repl.go:635,654,687) and instead
// read the panestream.SignalCenter projections added in Task 1
// (livenessCenter.Busy(session) / livenessCenter.Changed(session)).
//
// TDD contract: written BEFORE the migration lands. AC1/AC2 assert on
// StopEvent fields the CURRENT direct-call code already happens to satisfy in
// the positive case (so they may show pre-existing GREEN for the "value is
// right" half); AC3 is the discriminating RED test — it fails today because
// the checkpoint still calls the free functions directly. DO NOT MODIFY these
// tests — Builder migrates the callsites to make AC3 (and any currently-red
// case) pass without breaking AC1/AC2/AC4.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// checkpointPaneSeq builds the fakeTmux.paneSeq needed to land checkpoint1
// and checkpoint2 exactly, given how many CapturePane calls the driver makes
// around them. Empirically (traced via a logging TmuxController wrapper over
// this exact fixture/harness): boot consumes 2 captures (the marker-check
// read, then claude-tmux's tickDuringBoot auto-respond tick, which captures
// again internally but is not used for the marker decision); the post-paste
// baseline (driver_tmux_repl.go:499) consumes 1 more; each wait-loop
// iteration's auto-respond tick (autorespond.go:246) consumes 1 capture
// BEFORE a checkpoint fires, and the checkpoint's own rawPane read
// (driver_tmux_repl.go:616) consumes 1 more — but the interval elapses one
// full iteration late (elapsed=0 on the very first iteration never satisfies
// elapsed-intervalStart>=interval), so checkpoint 1 lands on the SIXTH
// capture (0-indexed position 5) and checkpoint 2 on the EIGHTH (position 7).
// The filler positions must stay a bare prompt marker — content the
// auto-responder's own capture never needs to react to.
func checkpointPaneSeq(cp1, cp2 string) []string {
	const filler = tmuxPromptMarkerDefault
	return []string{filler, filler, filler, filler, filler, cp1, filler, cp2}
}

// TestRunTmuxREPL_BusyFromCenter (AC1, positive): at the checkpoint,
// StopEvent.Busy must reflect the SignalCenter's Busy(session) projection
// (folded from panestream.PaneBusy) — true while a live-turn affordance is
// present, false once the pane goes quiet (no renderWedged in play).
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

// TestRunTmuxREPL_ProgressedFromCenter (AC2, positive): StopEvent.Progressed
// (evidence-only; zero decision consumers) must be sourced from the center's
// Changed(session) projection, which compares CONSECUTIVE Observe calls. The
// center's FIRST Observe call happens at checkpoint 1 (not at the pre-loop
// post-paste capture), so checkpoint 1 has no prior observation to compare
// against and must read Progressed=false; checkpoint 2 is the first real
// comparison and must reflect the genuine content delta between checkpoints
// 1 and 2.
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

// checkpointRegionSource extracts the stop-review checkpoint block from
// driver_tmux_repl.go, anchored on the two stable comment/line markers that
// bracket it, so a future reflow can't silently narrow (or widen) the scanned
// region without also updating this test.
func checkpointRegionSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not resolve this test file's path via runtime.Caller")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "driver_tmux_repl.go"))
	if err != nil {
		t.Fatalf("read driver_tmux_repl.go: %v", err)
	}
	lines := strings.Split(string(src), "\n")

	start, end := -1, -1
	for i, ln := range lines {
		if start == -1 && strings.Contains(ln, "Review checkpoint: a full interval elapsed") {
			start = i
		}
		if start != -1 && strings.Contains(ln, "intervalStart = elapsed") {
			end = i // keep scanning — take the LAST match, the outer-block close
			// (was "intervalBaselinePane = curPane" until that vestigial S4 dead
			// assignment was removed; the interval reset now closes the region)
		}
	}
	if start == -1 || end == -1 {
		t.Fatal("could not locate the stop-review checkpoint region markers in driver_tmux_repl.go")
	}
	return strings.Join(lines[start:end+1], "\n")
}

// TestRunTmuxREPL_NoDirectChromeParseAtCheckpoint (AC3, negative —
// discriminating anti-gaming test): the checkpoint block must no longer call
// panestream.PaneBusy( or PaneHasSubstantiveChange( directly. This is the
// test that defeats the "keep the direct call AND also call the center"
// cheapest fake — it fails today (RED) because driver_tmux_repl.go:635,654,687
// still call both functions inline.
func TestRunTmuxREPL_NoDirectChromeParseAtCheckpoint(t *testing.T) {
	region := checkpointRegionSource(t)
	for _, needle := range []string{"panestream.PaneBusy(", "PaneHasSubstantiveChange("} {
		if strings.Contains(region, needle) {
			t.Errorf("checkpoint region still calls %s directly — must read panestream.SignalCenter projections (Busy/Changed) instead", needle)
		}
	}
}

// TestRunTmuxREPL_S4MigrationUsesRealDeterministicReviewer (AC7,
// anti-gaming): the S4 migration corpus above must exercise the PRODUCTION
// deterministicReviewer type, not a stub standing in for the decision logic —
// same guard as TestWedgeCorpus_UsesRealDeterministicReviewer
// (signalcenter_wedge_invariant_test.go).
func TestRunTmuxREPL_S4MigrationUsesRealDeterministicReviewer(t *testing.T) {
	var r StopReviewer = newDeterministicReviewer(defaultArtifactMaxExtends)
	if _, ok := r.(deterministicReviewer); !ok {
		t.Fatalf("S4 migration corpus must exercise the real deterministicReviewer, got %T", r)
	}
}
