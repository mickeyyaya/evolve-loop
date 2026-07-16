package bridge

// driver_tmux_repl_cancel_test.go — session-lifecycle-verdict-clobbers residual
// (primary settle-retry fix c2258b72): the wait loop broke on ctx.Err() WITHOUT
// a final completion check, so a context-cancel landing AFTER the deliverable
// was already on disk (the next phase tearing down a finished session) was
// laundered into ExitArtifactTimeout — telemetry recorded a phase "timeout" for
// a benignly-torn-down COMPLETED session, and only the runner's settle-retry
// stood between that mislabel and a false FAIL. The fix: one final detector
// poll on cancellation (the artifact detector is a pure file stat, so it works
// under a dead ctx); a ready deliverable = a completed session = the normal
// success path, not a timeout.

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// ctxHonoringTmux wraps fakeTmux with production CapturePane semantics: a dead
// ctx cannot fork tmux (exec.CommandContext refuses pre-cancelled contexts), so
// the capture errors instead of returning canned data. Local to this file — it
// exists to prove the benign-cancel completion path still writes non-empty
// scrollback logs (the lastGoodPane fallback) when the real capture is
// impossible, which the ctx-blind shared fake cannot exercise.
type ctxHonoringTmux struct{ *fakeTmux }

func (c *ctxHonoringTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return c.fakeTmux.CapturePane(ctx, session, scrollback)
}

// TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout: the deliverable is
// written and the ctx cancelled in the SAME poll gap, so the cancel check runs
// before any poll ever observes the artifact. Pre-fix: break → !completed →
// ExitArtifactTimeout (the laundering). Post-fix: the final on-cancel poll sees
// the artifact → the session completes normally (ExitOK), and the scrollback
// logs carry the freshest observed pane (NOT empty — the dead ctx blocks the
// final capture, so the lastGoodPane fallback is the forensic record).
func TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &ctxHonoringTmux{&fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr bytes.Buffer
	fired := false
	sleep := func(time.Duration) {
		// First WAIT-LOOP tick: "prompt delivered" is printed immediately
		// before the loop's baseline capture, so a Sleep observing it is the
		// loop's own — the agent finishes its deliverable and the orchestrator
		// tears the session down, both inside one poll gap (after the baseline
		// pane was captured, matching the real teardown ordering).
		if !fired && strings.Contains(stderr.String(), "prompt delivered") {
			fired = true
			if err := os.WriteFile(fx.artifact, []byte("<!-- challenge-token: "+fx.token+" -->\nPROTOTYPE OK\n"), 0o644); err != nil {
				t.Fatalf("write artifact: %v", err)
			}
			cancel()
		}
	}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: sleep, LookupEnv: mapLookup(nil)})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)

	if !fired {
		t.Fatal("test harness defect: the cancel-after-deliverable injection never fired (boot marker not seen)")
	}
	if code == ExitArtifactTimeout {
		t.Fatalf("cancel-after-deliverable laundered into ExitArtifactTimeout (%d) — a finished session's teardown must not masquerade as a phase timeout; stderr=%q",
			code, stderr.String())
	}
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (ExitOK — deliverable was complete before the cancel); stderr=%q", code, ExitOK, stderr.String())
	}
	// Forensic evidence must survive the dead-ctx capture: the stderr log falls
	// back to the freshest pane observed during the wait (lastGoodPane), never
	// an empty write.
	logBytes, rerr := os.ReadFile(fx.stderrLog)
	if rerr != nil {
		t.Fatalf("read stderr log: %v", rerr)
	}
	if !strings.Contains(string(logBytes), tmuxPromptMarkerDefault) {
		t.Errorf("stderr log must carry the lastGoodPane fallback (freshest observed pane), got %q", string(logBytes))
	}
}

// TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut: the honest negative — a
// cancel with NO deliverable on disk keeps the ExitArtifactTimeout signal (the
// wait genuinely ended without completion; the runner's reconcile then decides
// what the teardown means). The final-poll fix must not invent completion.
func TestTmuxREPL_CancelWithoutDeliverable_StillTimesOut(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr bytes.Buffer
	fired := false
	sleep := func(time.Duration) {
		if !fired && strings.Contains(stderr.String(), "detected") {
			fired = true
			cancel() // teardown lands, deliverable never written
		}
	}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: sleep, LookupEnv: mapLookup(nil)})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass"), nil, &stdout, &stderr)

	if !fired {
		t.Fatal("test harness defect: the cancel injection never fired (boot marker not seen)")
	}
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout — no deliverable, the timeout signal is honest); stderr=%q",
			code, ExitArtifactTimeout, stderr.String())
	}
}
