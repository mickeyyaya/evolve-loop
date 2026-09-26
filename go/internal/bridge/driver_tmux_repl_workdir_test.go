package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// workdirRecordingTmux records the workdir the driver passed at session birth.
type workdirRecordingTmux struct {
	*FakeTmuxController
	bornIn string
}

func (w *workdirRecordingTmux) NewSessionIn(ctx context.Context, name string, width, height int, workdir string) error {
	w.bornIn = workdir
	return w.FakeTmuxController.NewSession(ctx, name, width, height)
}

func TestFleetModeRefusesEmptyWorktree(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.Worktree = ""
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}
	deps := fixtureDeps(tm)
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_FLEET": "1"})

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "fleet-refuse", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if code != ExitBadFlags {
		t.Errorf("code=%d, want ExitBadFlags(%d) — fleet mode must refuse a launch with no explicit worktree", code, ExitBadFlags)
	}
	if !errors.Is(err, errWorktreeRequired) {
		t.Errorf("err=%v, want errWorktreeRequired — the refusal must be typed, not a generic message", err)
	}
	for _, ev := range tm.Events {
		if strings.HasPrefix(ev, "new-session:") {
			t.Errorf("a tmux session was created (%s) despite the fleet-mode refusal — fail-closed means BEFORE any side effect", ev)
		}
	}
}

func TestSingleModeCwdFallbackWarnsLoudly(t *testing.T) {
	cfg := fixtureConfig(t)
	cfg.Worktree = ""
	var stderr bytes.Buffer
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}
	deps := fixtureDeps(tm)
	deps.Stderr = &stderr

	code, err := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "single-fallback", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil — single mode keeps the cwd fallback", code, err)
	}
	out := stderr.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "cwd") {
		t.Errorf("stderr lacks a loud cwd-fallback WARN; got:\n%s", out)
	}
	cwd, _ := os.Getwd()
	if !strings.Contains(out, cwd) {
		t.Errorf("the WARN must name the directory the agent will actually run in (%s); got:\n%s", cwd, out)
	}
}

func TestNewSessionBindsWorkdirAtBirth(t *testing.T) {
	cfg := fixtureConfig(t)
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}
	tm := &workdirRecordingTmux{FakeTmuxController: base}

	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "birth-bind", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil", code, err)
	}
	if tm.bornIn != cfg.Worktree {
		t.Errorf("session born in %q, want %q — a workdirSessionStarter controller must receive the workdir at new-session time (`tmux new-session -c`), not only via the later cd keystroke", tm.bornIn, cfg.Worktree)
	}
	// Belt-and-suspenders: the cd keystroke must STILL be sent even when the
	// session was born in the right directory.
	cdSent := false
	for _, ev := range tm.Events {
		if ev == "send:cd "+cfg.Worktree+"|true" {
			cdSent = true
		}
	}
	if !cdSent {
		t.Errorf("cd keystroke was not sent — CB.2 keeps it as the second layer; events=%v", tm.Events)
	}
}

// recipeWorkdirTmux gives the recipe-path fake the birth-bind capability.
type recipeWorkdirTmux struct {
	*fakeTmux
	bornIn string
}

func (w *recipeWorkdirTmux) NewSessionIn(ctx context.Context, name string, width, height int, workdir string) error {
	w.bornIn = workdir
	return w.fakeTmux.NewSession(ctx, name, width, height)
}

func TestRecipeFleetModeRefusesEmptyWorktree(t *testing.T) {
	deps := recipeDeps(&fakeTmux{})
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_FLEET": "1"})
	_, _, err := newRecipeDriver(&Config{Workspace: t.TempDir(), Agent: "recipe"}, deps, "claude-tmux")
	if !errors.Is(err, errWorktreeRequired) {
		t.Errorf("newRecipeDriver err=%v, want errWorktreeRequired — the recipe path shares the fleet fail-closed contract", err)
	}
}

func TestRecipeEnsureSessionBindsWorkdirAtBirth(t *testing.T) {
	wd := t.TempDir()
	tm := &recipeWorkdirTmux{fakeTmux: &fakeTmux{paneSeq: []string{"❯"}}}
	deps := recipeDeps(tm.fakeTmux)
	deps.Tmux = tm
	d := &recipeSessionDriver{
		cfg:        &Config{Workspace: t.TempDir(), Agent: "recipe"},
		deps:       deps,
		session:    "recipe-birth-bind",
		launchCmd:  "claude",
		workingDir: wd,
		marker:     "❯",
		scrollback: recipeBootScrollback,
	}
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if tm.bornIn != wd {
		t.Errorf("recipe session born in %q, want %q — EnsureSession must use the birth-bind capability when present", tm.bornIn, wd)
	}
}

func TestCapabilityAbsentFallsBackToPlainNewSession(t *testing.T) {
	cfg := fixtureConfig(t)
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}
	code, err := runTmuxREPL(context.Background(), cfg, fixtureDeps(tm), tmuxLaunch{
		name: "claude-tmux", session: "no-capability", launchCmd: "claude",
		promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if err != nil || code != ExitOK {
		t.Fatalf("runTmuxREPL = (%d,%v), want ExitOK,nil — capability-less controllers must keep working", code, err)
	}
	created := false
	for _, ev := range tm.Events {
		if strings.HasPrefix(ev, "new-session:") {
			created = true
		}
	}
	if !created {
		t.Fatalf("plain NewSession was not called; events=%v", tm.Events)
	}
}
