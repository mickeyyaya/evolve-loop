package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func (w *recipeEnvTmux) SessionEnvFingerprint(context.Context, string) (string, error) {
	return envFingerprint(w.bornEnv), nil
}

func TestNewSessionEnvArgs_CarriesNoValueOnlyTheFileAndTheFingerprint(t *testing.T) {
	t.Parallel()
	got := newSessionEnvArgs("s1", 200, 50, "/w", "/tmp/cell dir/env", "fp1")
	want := []string{
		"new-session", "-d", "-s", "s1", "-x", "200", "-y", "50", "-c", "/w",
		"/bin/sh", "-c", `exec /usr/bin/env -i TERM="$TERM" /bin/sh '/tmp/cell dir/env'`,
		";", "set-option", "-t", "=s1:", envFingerprintOption, "fp1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("args =\n  %q\nwant\n  %q", got, want)
	}
	bare := newSessionEnvArgs("s2", 1, 1, "", "/e/env", "fp")
	wantBare := []string{"new-session", "-d", "-s", "s2", "-x", "1", "-y", "1",
		"/bin/sh", "-c", `exec /usr/bin/env -i TERM="$TERM" /bin/sh /e/env`, ";", "set-option", "-t", "=s2:", envFingerprintOption, "fp"}
	if !slices.Equal(bare, wantBare) {
		t.Fatalf("no workdir means no -c workdir; got %q", bare)
	}
}

func TestWriteCellEnvFile_IsAPrivateScriptThatExportsSelfDeletesAndExecsTheShell(t *testing.T) {
	t.Parallel()
	file, err := writeCellEnvFile([]string{"TOKEN=it's secret", "PATH=/bin", "X;rm -rf ~=1"})
	if !filepath.IsAbs(file) {
		t.Fatalf("the script path must be absolute, since the pane starts in another dir: %q", file)
	}
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(file)
	defer os.RemoveAll(dir)
	info, err := os.Stat(file)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the env file must be 0600; got %v, %v", info, err)
	}
	if d, _ := os.Stat(dir); d.Mode().Perm() != 0o700 {
		t.Fatalf("the env dir must be 0700; got %v", d.Mode().Perm())
	}
	body, _ := os.ReadFile(file)
	want := "/bin/rm -rf " + shellQuotePOSIX(dir) + "\nset -a\nTOKEN='it'\\''s secret'\nPATH=/bin\nset +a\nexec /bin/sh\n"
	if string(body) != want {
		t.Fatalf("env file =\n%q\nwant\n%q", body, want)
	}
}

func TestWriteCellEnvFile_ReportsAnUnwritableTempDir(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	if _, err := writeCellEnvFile([]string{"A=1"}); err == nil {
		t.Fatal("a temp dir that cannot be made must fail the start, not fall back to argv")
	}
}

func TestEnvFingerprint_ChangesWithAnyNameOrValue(t *testing.T) {
	t.Parallel()
	a := envFingerprint([]string{"A=1", "B=2"})
	if a != envFingerprint([]string{"B=2", "A=1"}) || a == envFingerprint([]string{"A=1", "B=3"}) || a == envFingerprint([]string{"A=1"}) {
		t.Fatal("the fingerprint is order-free and changes with any name or value")
	}
}

func TestFakeTmuxController_NewSessionEnvRecordsTheSession(t *testing.T) {
	t.Parallel()
	f := &FakeTmuxController{}
	if err := f.NewSessionEnv(context.Background(), "s", 1, 1, "/w", []string{"A=1"}); err != nil || !f.Existing["s"] {
		t.Fatalf("NewSessionEnv = %v, existing=%v", err, f.Existing)
	}
}

func TestTmuxClientEnv_CarriesNoPassthroughValue(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", "/tmp/sockets")
	got := tmuxClientEnv([]string{"PATH=/bin", "LC_ALL=C", "CLAUDE_CODE_OAUTH_TOKEN=sk-x", "EVOLVE_PROJECT_ROOT=/r"})
	if !slices.Equal(got, []string{"PATH=/bin", "LC_ALL=C", "TMUX_TMPDIR=/tmp/sockets"}) {
		t.Fatalf("client env = %q", got)
	}
	os.Unsetenv("TMUX_TMPDIR")
	if got := tmuxClientEnv([]string{"PATH=/bin"}); !slices.Equal(got, []string{"PATH=/bin"}) {
		t.Fatalf("client env without TMUX_TMPDIR = %q", got)
	}
}

func TestFakeTmuxController_KeepsTheFingerprintOfEachSession(t *testing.T) {
	t.Parallel()
	f := &FakeTmuxController{}
	_ = f.NewSessionEnv(context.Background(), "s", 1, 1, "", []string{"A=1"})
	if got, err := f.SessionEnvFingerprint(context.Background(), "s"); err != nil || got != envFingerprint([]string{"A=1"}) {
		t.Fatalf("fingerprint = %q, %v", got, err)
	}
	if got, _ := f.SessionEnvFingerprint(context.Background(), "other"); got != "" {
		t.Fatalf("a session born without an env has no fingerprint; got %q", got)
	}
}

func TestReusedSession_AtEnforceOnlyASessionBornWithThisCellEnvIsReused(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	cfg := seatConfig("claude-tmux")
	deps := Deps{EnvStage: "enforce"}
	fake := &FakeTmuxController{}
	deps.Tmux = fake
	ctx := context.Background()
	_ = fake.NewSession(ctx, "old", 1, 1)
	if err := deps.checkReusedSessionEnv(ctx, cfg, "old"); err == nil || !strings.Contains(err.Error(), "not born with this cell environment") {
		t.Fatalf("a session born at shadow must not be reused at enforce; got %v", err)
	}
	_ = fake.NewSessionEnv(ctx, "same", 1, 1, "", append(deps.paneEnv(cfg), "BAD-NAME=1"))
	if err := deps.checkReusedSessionEnv(ctx, cfg, "same"); err != nil {
		t.Fatalf("a session born with this cell env can be reused: %v", err)
	}
	if err := (Deps{EnvStage: "shadow", Tmux: fake}).checkReusedSessionEnv(ctx, cfg, "old"); err != nil {
		t.Fatalf("at shadow, reuse is as today: %v", err)
	}
	if err := (Deps{EnvStage: "enforce", Tmux: struct{ TmuxController }{fake}}).checkReusedSessionEnv(ctx, cfg, "old"); err == nil {
		t.Fatal("a controller that cannot read a fingerprint cannot prove the env; refuse")
	}
	fake.FingerprintErr = errors.New("tmux gone")
	if err := deps.checkReusedSessionEnv(ctx, cfg, "same"); err == nil {
		t.Fatal("a fingerprint read error must refuse the reuse")
	}
}

func TestTmuxBoot_AtEnforceAReusedNamedSessionWithAnotherEnvIsRefused(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	tm := &FakeTmuxController{CaptureFrames: []string{"❯", "❯"}, Existing: map[string]bool{"named-cell": true}}
	cfg := fixtureConfig(t)
	cfg.CLI, cfg.Realization = "claude-tmux", RealizeFor("claude-tmux", LaunchIntent{})
	deps := fixtureDeps(tm)
	deps.EnvStage = "enforce"
	code, _ := runTmuxREPL(context.Background(), cfg, deps, tmuxLaunch{
		name: "claude-tmux", session: "named-cell", named: true, launchCmd: "claude", promptMarker: "❯", bootIntervalS: 1, bootOnly: true,
	})
	if code != ExitSafetyGate {
		t.Fatalf("exit = %d, want the safety gate for a named session born with another env", code)
	}
}

func TestRecipeEnsureSession_AtEnforceAReusedSessionWithAnotherEnvIsRefused(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	tm := &fakeTmux{paneSeq: []string{"❯"}}
	deps := recipeDeps(tm)
	deps.Tmux, deps.EnvStage = &recipeReuseTmux{fakeTmux: tm}, "enforce"
	d := &recipeSessionDriver{
		cfg:  &Config{Workspace: t.TempDir(), Agent: "recipe", CLI: "claude-tmux", Realization: RealizeFor("claude-tmux", LaunchIntent{})},
		deps: deps, session: "recipe-reuse", launchCmd: "claude", workingDir: t.TempDir(), marker: "❯", scrollback: recipeBootScrollback,
	}
	if err := d.EnsureSession(context.Background()); err == nil || !strings.Contains(err.Error(), "not born with this cell environment") {
		t.Fatalf("EnsureSession = %v, want a refusal", err)
	}
}

type recipeReuseTmux struct{ *fakeTmux }

func (recipeReuseTmux) HasSession(context.Context, string) bool { return true }

func (recipeReuseTmux) NewSessionEnv(context.Context, string, int, int, string, []string) error {
	return nil
}

func (recipeReuseTmux) SessionEnvFingerprint(context.Context, string) (string, error) {
	return "born-at-shadow", nil
}
