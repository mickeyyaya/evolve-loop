package bridge

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

func TestLiveSmokeTest_HealthyWritesArtifact(t *testing.T) {
	ws := t.TempDir()
	// The extra "working ❯" is the settling tick: the cross-poll stability window (completion.go) completes
	// the artifact one tick after it first appears, so the pane is captured once more before cleanup.
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}}
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

func TestLiveSmokeTest_NonTmuxDriverRejected(t *testing.T) {
	rc, _, _ := LiveSmokeTest(context.Background(), "claude-p", nil, liveSmokeDeps(&FakeTmuxController{}))
	if rc != ExitBadFlags {
		t.Fatalf("rc=%d, want ExitBadFlags for non-tmux driver", rc)
	}
}
