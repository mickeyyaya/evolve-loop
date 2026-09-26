package bridge

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// ctxHonoringTmux wraps fakeTmux with production CapturePane semantics: a
// dead ctx cannot fork tmux (exec.CommandContext refuses a cancelled
// context), so the capture errors instead of returning canned data. It
// exists to prove the benign-cancel completion path still falls back to
// lastGoodPane instead of writing empty logs — the ctx-blind shared fake
// cannot exercise that.
type ctxHonoringTmux struct{ *fakeTmux }

func (c *ctxHonoringTmux) CapturePane(ctx context.Context, session string, scrollback int) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return c.fakeTmux.CapturePane(ctx, session, scrollback)
}

// TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout writes the
// deliverable and cancels the ctx in the SAME poll gap, so the cancel check
// runs before any poll ever observes the artifact.
func TestTmuxREPL_CancelAfterDeliverable_CompletesNotTimeout(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &ctxHonoringTmux{&fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr bytes.Buffer
	fired := false
	sleep := func(time.Duration) {
		// The FIRST wait-loop Sleep follows "prompt delivered" on stderr
		// (printed right before the loop's baseline capture), so firing here
		// matches the real ordering: deliverable written, then teardown,
		// inside one poll gap.
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
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=17"), nil, &stdout, &stderr)

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
	logBytes, rerr := os.ReadFile(fx.stderrLog)
	if rerr != nil {
		t.Fatalf("read stderr log: %v", rerr)
	}
	if !strings.Contains(string(logBytes), tmuxPromptMarkerDefault) {
		t.Errorf("stderr log must carry the lastGoodPane fallback (freshest observed pane), got %q", string(logBytes))
	}
}

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
	code := eng.LaunchArgs(ctx, fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=17"), nil, &stdout, &stderr)

	if !fired {
		t.Fatal("test harness defect: the cancel injection never fired (boot marker not seen)")
	}
	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d (ExitArtifactTimeout — no deliverable, the timeout signal is honest); stderr=%q",
			code, ExitArtifactTimeout, stderr.String())
	}
	summary := artifactTimeoutSummary(stderr.String())
	for _, want := range []string{
		"cause=context_cancelled",
		`reason="context canceled"`,
		"phase=build",
		"cycle=17",
		"driver=claude-tmux",
		`artifact="artifact.md"`,
	} {
		if !strings.Contains(summary, want) {
			t.Errorf("cancel timeout summary missing %q; summary=%q", want, summary)
		}
	}
}

func TestTmuxREPL_ReviewerStopThenLateCancelKeepsReviewCause(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "")
	tmux := &fakeTmux{paneSeq: []string{tmuxPromptMarkerDefault}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callbackRan := false
	var stderr bytes.Buffer
	eng := newTestEngine(Deps{
		Tmux:               tmux,
		Sleep:              func(time.Duration) {},
		Reviewer:           &scriptedReviewer{verdicts: []ReviewVerdict{{Action: ReviewStop, Reason: "reviewer found terminal evidence"}}},
		ArtifactTimeoutS:   2,
		ArtifactMaxExtends: 1,
		OnStopReview: func(_, action, _ string) {
			if action == string(ReviewStop) {
				callbackRan = true
				cancel()
			}
		},
	})

	var stdout bytes.Buffer
	code := eng.LaunchArgs(ctx,
		fx.args("claude-tmux", "--allow-bypass", "--agent=build", "--cycle=41"), nil, &stdout, &stderr)

	if code != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want ExitArtifactTimeout; stderr=%q", code, stderr.String())
	}
	if !callbackRan {
		t.Fatal("test premise failed: stop-review callback did not cancel the context")
	}
	summary := artifactTimeoutSummary(stderr.String())
	if !strings.Contains(summary, "cause=review_stop") {
		t.Fatalf("late cancellation relabelled reviewer stop; summary=%q", summary)
	}
	if strings.Contains(summary, "cause=context_cancelled") {
		t.Fatalf("reviewer stop was misreported as cancellation; summary=%q", summary)
	}
}
