package bridge

// RED contract for LiveSmokeTest (cycle-283): the boot smoke-test passes
// against a quota-walled CLI because the wall only appears AFTER work is
// submitted. LiveSmokeTest is the probe that can actually see it: a real
// launch that submits one trivial contracted prompt and reports whether the
// REPL produced the artifact (healthy), hit a classified wall (pattern name
// from the escalation report), or failed otherwise.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func liveSmokeDeps(tmux TmuxController) Deps {
	return Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
	}
}

// liveArtifactTmux writes the live-smoke artifact when the prompt is pasted —
// the fake's stand-in for a healthy model answering the probe.
type liveArtifactTmux struct {
	*FakeTmuxController
	artifact string
}

func (a *liveArtifactTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := a.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	return os.WriteFile(a.artifact, []byte("OK\n"), 0o644)
}

// TestLiveSmokeTest_HealthyWritesArtifact: a REPL that boots and produces the
// artifact is healthy — ExitOK, no wall pattern.
func TestLiveSmokeTest_HealthyWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "done ❯", "cleanup"}}
	tm := &liveArtifactTmux{FakeTmuxController: base, artifact: filepath.Join(ws, LiveSmokeArtifact)}
	rc, pattern, _ := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, liveSmokeDeps(tm))
	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}
	if pattern != "" {
		t.Errorf("pattern=%q, want empty on healthy probe", pattern)
	}
	if tm.PasteCount == 0 {
		t.Error("live smoke must actually SUBMIT the trivial prompt (PasteBuffer never called)")
	}
}

// TestLiveSmokeTest_QuotaWallClassified: the cycle-283 replay — the pane shows
// the provider wall after submission; the autoresponder classifies rate_limit,
// escalates (85), and LiveSmokeTest surfaces the pattern name.
func TestLiveSmokeTest_QuotaWallClassified(t *testing.T) {
	ws := t.TempDir()
	wall := "■ You've hit your usage limit. Upgrade to Pro or try again at 6:11 AM."
	tm := &FakeTmuxController{CaptureFrames: []string{"›", "thinking", wall, wall, wall, wall, wall, wall}}
	rc, pattern, scrollback := LiveSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: ws}, liveSmokeDeps(tm))
	if rc != ExitUnknownPrompt {
		t.Fatalf("rc=%d, want ExitUnknownPrompt(85); scrollback tail: %s", rc, ScrollbackTail(scrollback, 4))
	}
	if pattern != "rate_limit" {
		t.Errorf("pattern=%q, want rate_limit (from the escalation report)", pattern)
	}
	if !strings.Contains(scrollback, "usage limit") {
		t.Errorf("scrollback must carry the wall text for reset-hint parsing; got %q", ScrollbackTail(scrollback, 4))
	}
}

// TestLiveSmokeTest_NonTmuxDriverRejected: only *-tmux drivers have a REPL to
// probe; anything else is a usage error, mirroring BootSmokeTest.
func TestLiveSmokeTest_NonTmuxDriverRejected(t *testing.T) {
	rc, _, _ := LiveSmokeTest(context.Background(), "claude-p", nil, liveSmokeDeps(&FakeTmuxController{}))
	if rc != ExitBadFlags {
		t.Fatalf("rc=%d, want ExitBadFlags for non-tmux driver", rc)
	}
}
