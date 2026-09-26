package bridge

import (
	"context"
	"testing"
)

// shellWedgeTmux reports a shell as the pane process after the prompt paste: the CLI exited and the paste spilled into the shell.
type shellWedgeTmux struct {
	*FakeTmuxController
	postPasteCmd string
}

func (s *shellWedgeTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := s.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	s.PaneCmd = s.postPasteCmd
	return nil
}

// markerOverDeadShell is a stale prompt marker in scrollback above a wedged zsh continuation prompt.
const markerOverDeadShell = `❯ previous output above
user@host evolve-loop % knowledge-base/research/
bquote>
zsh: command not found: and`

func TestBootRejectsMarkerOverDeadShell(t *testing.T) {
	cfg := fixtureConfig(t)
	frames := make([]string, 0, tmuxREPLBootTimeoutS+1)
	for i := 0; i < tmuxREPLBootTimeoutS+1; i++ {
		frames = append(frames, markerOverDeadShell)
	}
	tm := &FakeTmuxController{CaptureFrames: frames, PaneCmd: "zsh"}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "handshake-deadshell", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true, guardDeadShell: true,
	})
	if err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}
	if code != ExitREPLBootTimeout {
		t.Fatalf("RED (cycle-274): marker over a dead shell declared READY (code=%d) — "+
			"the boot predicate must reject a pane whose foreground process is a shell; want ExitREPLBootTimeout (%d)",
			code, ExitREPLBootTimeout)
	}
}

func TestBootAcceptsMarkerWithCLIProcess(t *testing.T) {
	cfg := fixtureConfig(t)
	// Two frames: boot marker + the deferred cleanup's final-scrollback capture.
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯ done"}, PaneCmd: "node"}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "handshake-ok", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true, guardDeadShell: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK — a CLI-process pane with the marker is ready", code, err)
	}
}

func TestPostPasteShellSpillFailsFast(t *testing.T) {
	cfg := fixtureConfig(t)
	// Frames: boot marker (process node), then the post-paste interval baseline showing the spill (process zsh),
	// then the deferred-cleanup final scrollback. No artifact is ever written.
	base := &FakeTmuxController{
		CaptureFrames: []string{"❯", cycle274BquoteSpill, cycle274BquoteSpill},
		PaneCmd:       "node",
	}
	tm := &shellWedgeTmux{FakeTmuxController: base, postPasteCmd: "zsh"}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "handshake-spill", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, guardDeadShell: true,
	})
	if err != nil {
		t.Fatalf("runTmuxREPL err: %v", err)
	}
	if code != ExitREPLBootTimeout {
		t.Fatalf("RED (cycle-274): prompt pasted into a dead shell did not fail fast (code=%d) — "+
			"want ExitREPLBootTimeout (%d) so the fallback chain takes over instead of a 25-min wedge",
			code, ExitREPLBootTimeout)
	}
}

func TestPostPasteSpillLookalikeWithCLIProcessIgnored(t *testing.T) {
	cfg := fixtureConfig(t)
	// The pane quotes spill text while the foreground process is still the CLI, so the run must complete.
	// The lookalike frame is queued twice: the cross-poll stability window captures the settled pane once more.
	base := &FakeTmuxController{
		CaptureFrames: []string{"❯", cycle274BquoteSpill + "\n❯", cycle274BquoteSpill + "\n❯", "final", "cleanup"},
		PaneCmd:       "node",
	}
	tm := &artifactOnPasteTmux{FakeTmuxController: base, artifact: cfg.Artifact}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "handshake-lookalike", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, guardDeadShell: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK — spill-lookalike text with a live CLI process must be ignored (process check is authoritative)", code, err)
	}
}

// A harness whose REPL is a shell script (the RealTmux integration fixtures) must boot when guardDeadShell is unset.
func TestGuardOffShellREPLBoots(t *testing.T) {
	cfg := fixtureConfig(t)
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯ done"}, PaneCmd: "bash"}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "itest-tmux", session: "handshake-guardoff", launchCmd: "/tmp/fake-repl.sh",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK — guardDeadShell off must keep shell-script harnesses bootable (Ubuntu CI regression)", code, err)
	}
}

func TestIsShellProcess(t *testing.T) {
	cases := map[string]bool{
		"zsh": true, "-zsh": true, "bash": true, "-bash": true, "sh": true,
		"fish": true, "dash": true, "tcsh": true, "ksh": true,
		"node": false, "codex": false, "claude": false, "": false, "python3": false,
	}
	for in, want := range cases {
		if got := isShellProcess(in); got != want {
			t.Errorf("isShellProcess(%q) = %v, want %v", in, got, want)
		}
	}
}
