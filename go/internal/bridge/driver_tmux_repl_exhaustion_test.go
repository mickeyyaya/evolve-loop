package bridge

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// driver_tmux_repl_exhaustion_test.go — the wait-loop fast-fail (S2). A CLI that
// hits its quota/rate-limit MID-PHASE renders the manifest's exhausted_regex
// wall and parks at its prompt without exiting. The SignalCenter's ExhaustionProbe
// now reports LivenessExhausted for that pane; the wait loop must return
// ExitUnknownPrompt (85) so the dispatch chain fails over — NOT burn the full
// artifact timeout (81) while nudging a walled CLI (the agy hang-forever livelock).

// A pane matching the CLI's exhausted_regex fast-fails to ExitUnknownPrompt.
func TestTmuxREPL_ExhaustionWall_FailsOverFast(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Boots (prompt marker ❯ present) AND shows the quota wall. This is the
	// EXACT wall captured in cycles 904–911's audit-escalation-report.json —
	// the per-model wording ("your Fable 5 limit") that the pre-fix
	// exhausted_regex ("reached your (usage|weekly) limit") did NOT match, so
	// the fast-fail was bypassed and 8 audit cycles burned the full artifact
	// timeout. The artifact is never written, so only the exhaustion override
	// can end the run.
	walled := "❯\nYou've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model.\n❯"
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{walled}}}
	code, stderr := runTmuxNudge(t, fx, tmux)
	if code != ExitUnknownPrompt {
		t.Fatalf("exit = %d, want %d (ExitUnknownPrompt — exhaustion fast-fail, not %d ExitArtifactTimeout); stderr=%q",
			code, ExitUnknownPrompt, ExitArtifactTimeout, stderr)
	}
	// Fast path: the ~2s poll must catch the wall BEFORE any 300s stop-review
	// checkpoint runs (the production gap — the checkpoint-only path recovered at
	// the 300s interval, not immediately), and BEFORE the walled CLI is nudged.
	if strings.Contains(stderr, "stop-review") {
		t.Errorf("exhaustion must fast-fail on the poll loop before any stop-review checkpoint; stderr=%q", stderr)
	}
	if n := tmux.deliveriesNaming(fx.artifact); n != 0 {
		t.Errorf("a walled CLI must NOT be nudged (%d nudge deliveries) — the fast-fail preempts the nudge", n)
	}
}

// The exhaustion check must run on the FAST poll loop (~2s cadence), not only
// the 300s stop-review checkpoint: a walled pane fails over in a handful of poll
// iterations, NOT the ~150 it takes for elapsed to reach the review interval.
// (The checkpoint-only path recovered — but at the 300s interval, which in
// production is a 5-minute hang per walled phase.) Counts deps.Sleep calls (one
// per poll) as the iteration proxy.
func TestTmuxREPL_ExhaustionWall_FastFailNotAtCheckpoint(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	// Real per-model wall wording (cycle-910/911 capture), as above.
	walled := "❯\nYou've reached your Fable 5 limit. Run /usage-credits to continue or switch models with /model.\n❯"
	tmux := &nudgeRecordingTmux{fakeTmux: &fakeTmux{paneSeq: []string{walled}}}
	polls := 0
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) { polls++ }, LookupEnv: mapLookup(nil)})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)
	if code != ExitUnknownPrompt {
		t.Fatalf("exit = %d, want %d (ExitUnknownPrompt); stderr=%q", code, ExitUnknownPrompt, stderr.String())
	}
	if polls > 10 {
		t.Errorf("exhaustion fast-fail took %d poll iterations — must catch the wall on the fast loop, not wait ~150 iters for the 300s checkpoint", polls)
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
