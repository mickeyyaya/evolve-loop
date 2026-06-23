// boot_handshake_test.go — RED contract for inbox
// codex-update-menu-swallows-injection (2026-06-10T11-10Z, 3× recurrence
// cycles 274/277): the boot loop declared "REPL ready" on a prompt-marker
// SUBSTRING alone. A marker lookalike rendered over a dead shell (codex's
// update menu exited to zsh; the pasted prompt spilled into quote>/bquote>
// continuation) read as ready, the injection landed in the shell, and the
// phase wedged 25+ min behind a false-positive liveness signal.
//
// Contract pinned here (R3.1/R3.2 of the concurrency-factory plan):
//
//  1. Marker visible but the pane's foreground process is a SHELL → NOT
//     ready. The shell set is closed (zsh/bash/…); CLI binary names vary
//     (claude runs under node), so the predicate rejects-known-shell rather
//     than requires-known-binary — degradation-safe for controllers that
//     don't implement PaneCommander.
//  2. Post-paste: the first wait-loop pane (the existing interval baseline,
//     no new capture) showing shell-spill signatures WITH shell-process
//     confirmation → fail fast with ExitREPLBootTimeout (transient → the
//     fallback chain), never a 25-min stall. Mid-run process death stays
//     the observer's job (plan R3.4).
//  3. A pane that merely CONTAINS spill-lookalike text (e.g. a prompt
//     discussing shell errors) while the process is the CLI → ignored.
package bridge

import (
	"context"
	"testing"
)

// shellWedgeTmux is a FakeTmuxController whose reported pane process flips
// to a shell after the prompt paste — the exact cycle-274 sequence (codex
// exited to zsh; the paste spilled into the shell).
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

// markerOverDeadShell renders the cycle-274 trap: a stale prompt marker in
// scrollback above a wedged zsh continuation prompt.
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
	// Frames: boot sees the marker (process=node, ready); the post-paste
	// interval baseline shows the spill (process now zsh). No artifact is
	// ever written — pre-fix this stalls through the whole artifact wait;
	// post-fix it returns ExitREPLBootTimeout on the baseline check.
	// Frame slots: boot-marker → post-paste interval baseline (the spill) →
	// deferred-cleanup final scrollback.
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
	// The pane QUOTES spill text (an agent discussing shell errors) but the
	// foreground process is still the CLI → must not fail. The artifact
	// appears on paste, so the run completes normally.
	base := &FakeTmuxController{
		CaptureFrames: []string{"❯", cycle274BquoteSpill + "\n❯", "final", "cleanup"},
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

// TestGuardOffShellREPLBoots pins the opt-out (the PR-71 Ubuntu CI
// regression): a harness whose "REPL" legitimately IS a shell script (the
// RealTmux integration fixtures) must boot normally when guardDeadShell is
// unset, even though the pane process reports a shell. Contrast:
// TestBootRejectsMarkerOverDeadShell covers the armed+shell→reject mirror.
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
