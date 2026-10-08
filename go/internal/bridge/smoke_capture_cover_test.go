package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

func TestSmokeTests_AnUnmakeableScratchWorkspaceIsABadFlagAndLaunchesNothing(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "absent"))
	boot := &fakeTmux{paneSeq: []string{"❯"}}
	bootDeps, _ := bootSmokeDeps(boot)
	live := &FakeTmuxController{CaptureFrames: []string{"❯"}}

	bootRC, _ := BootSmokeTest(context.Background(), "claude-tmux", &Config{}, bootDeps)
	liveRC, _, _ := LiveSmokeTest(context.Background(), "claude-tmux", &Config{}, liveSmokeDeps(live))

	if bootRC != ExitBadFlags || len(boot.existing) != 0 {
		t.Errorf("BootSmokeTest rc=%d sessions=%v, want ExitBadFlags and no session", bootRC, boot.existing)
	}
	if liveRC != ExitBadFlags || live.PasteCount != 0 {
		t.Errorf("LiveSmokeTest rc=%d pastes=%d, want ExitBadFlags and no prompt", liveRC, live.PasteCount)
	}
}

func TestLiveSmokeTest_AMinimalConfigOwnsAndRemovesItsScratchWorkspace(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}}
	tm := &scratchArtifactTmux{FakeTmuxController: base}

	rc, _, _ := LiveSmokeTest(context.Background(), "claude-tmux", &Config{}, liveSmokeDeps(tm))

	if rc != ExitOK || base.PasteCount == 0 || tm.workspace == "" || !strings.HasPrefix(tm.workspace, tmp) {
		t.Errorf("rc=%d pastes=%d workspace=%q, want a healthy probe in a scratch workspace under %s", rc, base.PasteCount, tm.workspace, tmp)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Errorf("TMPDIR entries = %v, %v, want the scratch workspace removed after the probe", entries, err)
	}
}

func TestLiveSmokeTest_AnUnwritablePromptIsABadFlag(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.Mkdir(filepath.Join(ws, "live-smoke-prompt.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	tm := &FakeTmuxController{CaptureFrames: []string{"❯"}}

	rc, _, _ := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, liveSmokeDeps(tm))

	if rc != ExitBadFlags || tm.PasteCount != 0 {
		t.Errorf("rc=%d pastes=%d, want ExitBadFlags and no prompt", rc, tm.PasteCount)
	}
}

type scratchArtifactTmux struct {
	*FakeTmuxController
	workspace string
}

func (a *scratchArtifactTmux) LoadBuffer(ctx context.Context, session, path string) error {
	for dir := filepath.Dir(path); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "live-smoke-prompt.txt")); err == nil {
			a.workspace = dir
			break
		}
	}
	return a.FakeTmuxController.LoadBuffer(ctx, session, path)
}

func (a *scratchArtifactTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := a.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.workspace, LiveSmokeArtifact), []byte("OK\n"), 0o644)
}

type pickerTmux struct {
	*fakeTmux
	loadErr, enterErr error
}

func (p *pickerTmux) LoadBuffer(ctx context.Context, session, path string) error {
	if p.loadErr != nil {
		return p.loadErr
	}
	return p.fakeTmux.LoadBuffer(ctx, session, path)
}

func (p *pickerTmux) SendKeys(ctx context.Context, session, keys string, enter bool) error {
	if keys == "Enter" && !enter && p.enterErr != nil {
		return p.enterErr
	}
	return p.fakeTmux.SendKeys(ctx, session, keys, enter)
}

func TestCaptureModelPicker_AnEphemeralSessionThatCannotStartIsReaped(t *testing.T) {
	t.Parallel()
	tx := &fakeTmux{newSessErr: errors.New("server exited")}
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "models", Realization: RealizeFor("claude-tmux", LaunchIntent{})}
	deps := recipeDeps(tx)
	deps.ListProcesses = failingList(errors.New("unused"))
	deps.SignalProcess = nil

	_, err := CaptureModelPicker(context.Background(), cfg, deps, "claude-tmux")

	if err == nil || !strings.HasPrefix(err.Error(), "recipe: ensure session: new-session: ") || !strings.Contains(err.Error(), "server exited") {
		t.Errorf("err = %v, want the ensure-session error", err)
	}
}

func TestCaptureModelPicker_AFailedSendIsReturned(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tx   *pickerTmux
		want string
	}{
		{&pickerTmux{fakeTmux: &fakeTmux{paneSeq: []string{"❯"}}, loadErr: errors.New("no buffer")}, "inject load-buffer: no buffer"},
		{&pickerTmux{fakeTmux: &fakeTmux{paneSeq: []string{"❯"}}, enterErr: errors.New("session gone")}, "recipe: /model extra-enter for claude-tmux: session gone"},
	}
	for i, tc := range cases {
		name := "mpx" + string(rune('a'+i))
		tc.tx.existing = map[string]bool{NamedSessionName(name): true}
		deps := covDeps()
		deps.Tmux = tc.tx

		_, err := CaptureModelPicker(context.Background(), namedCaptureCfg(t, name), deps, "claude-tmux")

		if err == nil || err.Error() != tc.want {
			t.Errorf("case %d: err = %v, want %q", i, err, tc.want)
		}
	}
}

func TestNewController_TheCaptureRunsTheMappedCommandInTheFamilySession(t *testing.T) {
	t.Parallel()
	name := "ctl1"
	tx := &fakeTmux{existing: map[string]bool{NamedSessionName(name): true}, paneSeq: []string{"Usage: 12% of weekly limit ❯"}}
	deps := covDeps()
	deps.Tmux = tx
	ctrl := NewController(&Config{Workspace: t.TempDir(), Agent: "control", SessionName: name}, deps).(*cliController)
	ctrl.resolve = func(cli string) (Manifest, error) {
		return Manifest{CLI: cli, Binary: "claude", PromptMarker: "❯", Controls: map[string]ControlSpec{"usage": {Send: "/usage", Await: "prompt_marker"}}}, nil
	}

	resp, err := ctrl.Do(context.Background(), "claude", clicontrol.EventUsage)

	if err != nil || !strings.Contains(resp.Pane, "12%") {
		t.Fatalf("Do = %+v, %v, want the captured usage pane", resp, err)
	}
	if !tx.sentContains("") || len(tx.sentSeq) == 0 || !strings.Contains(strings.Join(tx.sentSeq, ","), "paste-buffer") {
		t.Errorf("sent = %v, want the /usage command pasted into the session", tx.sentSeq)
	}
}
