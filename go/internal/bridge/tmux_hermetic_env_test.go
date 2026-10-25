package bridge

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
)

type envRecordingTmux struct {
	*FakeTmuxController
	bornIn  string
	bornEnv []string
}

func (w *envRecordingTmux) NewSessionIn(ctx context.Context, name string, width, height int, workdir string) error {
	w.bornIn = workdir
	return w.FakeTmuxController.NewSession(ctx, name, width, height)
}

func (w *envRecordingTmux) NewSessionEnv(ctx context.Context, name string, width, height int, workdir string, env []string) error {
	w.bornIn, w.bornEnv = workdir, env
	return w.FakeTmuxController.NewSession(ctx, name, width, height)
}

func bootPane(t *testing.T, stage string, tm TmuxController) (int, error) {
	t.Helper()
	cfg := fixtureConfig(t)
	cfg.CLI, cfg.Realization = "claude-tmux", RealizeFor("claude-tmux", LaunchIntent{})
	deps := fixtureDeps(tm)
	deps.EnvStage = stage
	return runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "hermetic-" + stage, launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
}

func TestTmuxBoot_AtEnforceThePaneIsBornWithTheCellEnvOnly(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	tm := &envRecordingTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}}
	if code, err := bootPane(t, "enforce", tm); code != ExitOK || err != nil {
		t.Fatalf("boot = %d, %v", code, err)
	}
	if tm.bornEnv == nil || tm.bornIn == "" {
		t.Fatalf("the pane must be born through NewSessionEnv in its workdir; env=%v dir=%q", tm.bornEnv, tm.bornIn)
	}
	names := envNames(tm.bornEnv)
	for leak := range launcherLeaks {
		if slices.Contains(names, leak) {
			t.Errorf("launcher variable %s reached the pane", leak)
		}
	}
	if !slices.Contains(tm.bornEnv, "CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false") || !slices.Contains(tm.bornEnv, "HOME=/Users/cell") {
		t.Errorf("the pane lost its manifest or base variables: %v", names)
	}
}

func TestTmuxBoot_InShadowThePaneIsBornAsToday(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	tm := &envRecordingTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}}
	if code, err := bootPane(t, "", tm); code != ExitOK || err != nil {
		t.Fatalf("boot = %d, %v", code, err)
	}
	if tm.bornEnv != nil || tm.bornIn == "" {
		t.Fatalf("shadow must use today's session start; env=%v dir=%q", tm.bornEnv, tm.bornIn)
	}
}

func TestTmuxBoot_AtEnforceAControllerWithoutAnEnvStartRefusesTheLaunch(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	tm := struct{ TmuxController }{&FakeTmuxController{CaptureFrames: []string{"❯", "❯"}}}
	code, err := bootPane(t, "enforce", tm)
	if code != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "allowlisted environment") {
		t.Fatalf("a pane that cannot be born clean must not start; got %d, %v", code, err)
	}
}

func TestRecipeEnsureSession_AtEnforceThePaneIsBornWithTheCellEnvOnly(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	tm := &recipeEnvTmux{fakeTmux: &fakeTmux{paneSeq: []string{"❯"}}}
	deps := recipeDeps(tm.fakeTmux)
	deps.Tmux, deps.EnvStage = tm, "enforce"
	d := &recipeSessionDriver{
		cfg:  &Config{Workspace: t.TempDir(), Agent: "recipe", CLI: "claude-tmux", Realization: RealizeFor("claude-tmux", LaunchIntent{})},
		deps: deps, session: "recipe-hermetic", launchCmd: "claude", workingDir: t.TempDir(), marker: "❯", scrollback: recipeBootScrollback,
	}
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("EnsureSession: %v", err)
	}
	if tm.bornEnv == nil || slices.Contains(envNames(tm.bornEnv), "FOO_LEAK") {
		t.Fatalf("the recipe pane must be born with the cell env only; env=%v", tm.bornEnv)
	}
}

type recipeEnvTmux struct {
	*fakeTmux
	bornEnv []string
}

func (w *recipeEnvTmux) NewSessionEnv(ctx context.Context, name string, width, height int, _ string, env []string) error {
	w.bornEnv = env
	return w.fakeTmux.NewSession(ctx, name, width, height)
}

func TestNewSessionEnvArgs_SetsEachPairAndStartsAShellWithOnlyThoseNames(t *testing.T) {
	t.Parallel()
	got := newSessionEnvArgs("s1", 200, 50, "/w", []string{"PATH=/bin", "HOME=/h", "PATH=/usr/bin"})
	want := []string{
		"new-session", "-d", "-s", "s1", "-x", "200", "-y", "50", "-c", "/w",
		"-e", "PATH=/bin", "-e", "HOME=/h", "-e", "PATH=/usr/bin",
		"/bin/sh", "-c", `exec env -i HOME="$HOME" PATH="$PATH" /bin/sh`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("args =\n  %q\nwant\n  %q", got, want)
	}
	if bare := newSessionEnvArgs("s2", 1, 1, "", nil); !slices.Equal(bare, []string{"new-session", "-d", "-s", "s2", "-x", "1", "-y", "1", "/bin/sh", "-c", "exec env -i /bin/sh"}) {
		t.Fatalf("no workdir means no -c, and no env means an empty environment; got %q", bare)
	}
}

func TestFakeTmuxController_NewSessionEnvRecordsTheSession(t *testing.T) {
	t.Parallel()
	f := &FakeTmuxController{}
	if err := f.NewSessionEnv(context.Background(), "s", 1, 1, "/w", []string{"A=1"}); err != nil || !f.Existing["s"] {
		t.Fatalf("NewSessionEnv = %v, existing=%v", err, f.Existing)
	}
}

func TestTmuxClientEnv_IsTheCellEnvPlusTheSocketDirectoryOnly(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	t.Setenv("TMUX_TMPDIR", "/tmp/sockets")
	if got := tmuxClientEnv([]string{"PATH=/bin"}); !slices.Equal(got, []string{"PATH=/bin", "TMUX_TMPDIR=/tmp/sockets"}) {
		t.Fatalf("client env = %q", got)
	}
	os.Unsetenv("TMUX_TMPDIR")
	if got := tmuxClientEnv([]string{"PATH=/bin"}); !slices.Equal(got, []string{"PATH=/bin"}) {
		t.Fatalf("client env without TMUX_TMPDIR = %q", got)
	}
}
