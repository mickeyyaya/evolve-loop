package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	cycle274CodexUpdateMenu = `✨ Update available! 0.138.0 -> 0.139.0

› 1. Update now (runs ` + "`brew upgrade --cask codex`" + `)
  2. Skip
  3. Skip until next version

  Press enter to continue`

	cycle274BquoteSpill = `user@host evolve-loop % knowledge-base/research/
bquote>
bquote> # Evolve Architecture Designer
zsh: command not found: and`
)

type artifactOnPasteTmux struct {
	*FakeTmuxController
	artifact string
}

func (a *artifactOnPasteTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := a.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	return os.WriteFile(a.artifact, []byte("done\n"), 0o644)
}

func fixtureConfig(t *testing.T) *Config {
	t.Helper()
	ws := t.TempDir()
	prompt := filepath.Join(ws, "prompt.txt")
	if err := os.WriteFile(prompt, []byte("write the artifact\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	return &Config{
		Workspace:        ws,
		Worktree:         ws,
		ProjectRoot:      ws,
		PromptFile:       prompt,
		Artifact:         filepath.Join(ws, "artifact.md"),
		StdoutLog:        filepath.Join(ws, "stdout.log"),
		StderrLog:        filepath.Join(ws, "stderr.log"),
		Agent:            "build",
		ArtifactTimeoutS: 1,
	}
}

func fixtureDeps(tm TmuxController) Deps {
	return Deps{
		Tmux:      tm,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(map[string]string{"EVOLVE_PHASE_RECOVERY": "off"}),
		Stderr:    os.Stderr,
	}.withDefaults()
}

func TestTmuxFixture(t *testing.T) {
	t.Run("boot_success", func(t *testing.T) {
		cfg := fixtureConfig(t)
		tm := &FakeTmuxController{CaptureFrames: []string{"❯", cycle274BquoteSpill}}
		code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
			name: "claude-tmux", session: "fixture-boot", launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
		})
		if err != nil || code != ExitOK {
			t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil", code, err)
		}
	})

	t.Run("boot_timeout_negative", func(t *testing.T) {
		cfg := fixtureConfig(t)
		frames := make([]string, 0, tmuxREPLBootTimeoutS+1)
		for i := 0; i < tmuxREPLBootTimeoutS+1; i++ {
			frames = append(frames, cycle274BquoteSpill)
		}
		tm := &FakeTmuxController{CaptureFrames: frames}
		code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
			name: "claude-tmux", session: "fixture-timeout", launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1,
		})
		if err != nil || code != ExitREPLBootTimeout {
			t.Fatalf("runTmuxREPL = (%d,%v), want ExitREPLBootTimeout,nil", code, err)
		}
	})

	t.Run("artifact_delivery", func(t *testing.T) {
		cfg := fixtureConfig(t)
		base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "final scrollback", "cleanup scrollback"}}
		tm := &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}
		code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
			name: "claude-tmux", session: "fixture-artifact", launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1,
		})
		if err != nil || code != ExitOK {
			t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil", code, err)
		}
		if tm.PasteCount != 1 {
			t.Fatalf("PasteBuffer count = %d, want 1", tm.PasteCount)
		}
	})
}

func TestFakeTmuxControllerPanicsOnUnderrun(t *testing.T) {
	tm := &FakeTmuxController{}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("CapturePane without a queued frame must panic")
		}
	}()
	_, _ = tm.CapturePane(context.Background(), "s", 0)
}

func TestCodexUpdateMenuDismiss(t *testing.T) {
	tests := []struct {
		name     string
		frames   []string
		wantSkip bool
	}{
		{
			name:     "menu_present_skip_before_inject",
			frames:   []string{cycle274CodexUpdateMenu, "ready ›", "idle ›", "final", "cleanup"},
			wantSkip: true,
		},
		{
			name:     "menu_absent_no_spurious_skip",
			frames:   []string{"ready ›", "idle ›", "final", "cleanup"},
			wantSkip: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := fixtureConfig(t)
			base := &FakeTmuxController{CaptureFrames: append([]string(nil), tc.frames...)}
			tm := &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}
			code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
				name: "codex-tmux", session: "codex-update", launchCmd: "codex", promptMarker: "›",
				bootIntervalS: 1, bootMenuSkip: "2",
			})
			if err != nil || code != ExitOK {
				t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil", code, err)
			}
			skipAt, pasteAt := eventIndex(tm.Events, "send:2|true"), eventIndex(tm.Events, "paste-buffer")
			if tc.wantSkip {
				if skipAt < 0 {
					t.Fatalf("Skip keypress not sent; events=%v", tm.Events)
				}
				if pasteAt < 0 || skipAt > pasteAt {
					t.Fatalf("Skip must precede paste; events=%v", tm.Events)
				}
			} else if skipAt >= 0 {
				t.Fatalf("unexpected Skip keypress without menu; events=%v", tm.Events)
			}
			if strings.Count(strings.Join(tm.Events, "\n"), "paste-buffer") != 1 {
				t.Fatalf("prompt should be pasted exactly once; events=%v", tm.Events)
			}
		})
	}
}

func eventIndex(events []string, want string) int {
	for i, ev := range events {
		if ev == want {
			return i
		}
	}
	return -1
}
