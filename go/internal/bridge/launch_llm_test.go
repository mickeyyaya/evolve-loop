package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type recordedExec struct {
	name, dir string
	args, env []string
}

func recordingRunner(rec *recordedExec) CmdRunner {
	return func(_ context.Context, name, dir string, args, env []string, _ io.Reader, _, _ io.Writer) (int, error) {
		*rec = recordedExec{name: name, dir: dir, args: args, env: env}
		return 0, nil
	}
}

var runTmuxEnv = map[string]string{"TMUX": "/tmp/tmux-501/run,1,0", "TMUX_PANE": "%3"}

func TestLaunchLLM_EachExecPurposeKeepsCLAUDECODEFromTheChildAtEnforce(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	for k, v := range runTmuxEnv {
		t.Setenv(k, v)
	}
	for _, purpose := range []LaunchPurpose{PurposeProbe, PurposeUpdate, PurposeInventory, PurposeModelQuery} {
		var rec recordedExec
		var stderr bytes.Buffer
		deps := Deps{EnvStage: "enforce", Runner: recordingRunner(&rec), Stderr: &stderr}

		rc, err := LaunchLLM(context.Background(), deps, LLMExec{Purpose: purpose, Binary: "claude", Args: []string{"--version"}, Dir: "/w"})

		if rc != 0 || err != nil {
			t.Fatalf("%s: rc=%d err=%v", purpose, rc, err)
		}
		if names := envNames(rec.env); slices.Contains(names, "CLAUDECODE") || slices.Contains(names, "FOO_LEAK") || !slices.Contains(names, "PATH") {
			t.Errorf("%s: child env names %v; want the allowlist without CLAUDECODE", purpose, names)
		}
		if rec.name != "claude" || rec.dir != "/w" || !slices.Equal(rec.args, []string{"--version"}) {
			t.Errorf("%s: ran %+v", purpose, rec)
		}
	}
}

func TestLaunchLLM_AtShadowTheChildKeepsTodaysEnvExceptTheRunTmux(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	for k, v := range runTmuxEnv {
		t.Setenv(k, v)
	}
	var rec recordedExec
	deps := Deps{EnvStage: "shadow", Runner: recordingRunner(&rec), Stderr: io.Discard}

	if _, err := LaunchLLM(context.Background(), deps, LLMExec{Purpose: PurposeInventory, Binary: "codex", Args: []string{"--version"}}); err != nil {
		t.Fatal(err)
	}

	names := envNames(rec.env)
	if !slices.Contains(names, "CLAUDECODE") || !slices.Contains(names, "FOO_LEAK") {
		t.Errorf("at shadow the child keeps today's env; names %v", names)
	}
	if slices.Contains(names, "TMUX") || slices.Contains(names, "TMUX_PANE") {
		t.Errorf("each purpose unsets TMUX as the phase pane does; names %v", names)
	}
}

func TestLaunchLLM_DropsTheNamedVariablesAfterTheManifestDefaults(t *testing.T) {
	t.Parallel()
	var rec recordedExec
	deps := Deps{EnvStage: "off", Runner: recordingRunner(&rec), Stderr: io.Discard}

	if _, err := LaunchLLM(context.Background(), deps, LLMExec{Purpose: PurposeUpdate, Binary: "agy", Args: []string{"update"}, Drop: []string{"AGY_CLI_DISABLE_AUTO_UPDATE"}}); err != nil {
		t.Fatal(err)
	}

	if slices.Contains(envNames(rec.env), "AGY_CLI_DISABLE_AUTO_UPDATE") {
		t.Errorf("the updater must not see the auto-update off switch that the agy manifest sets; env names %v", envNames(rec.env))
	}
}

func TestLaunchLLM_LogsTheManifestReasonForAnUnsandboxedExec(t *testing.T) {
	t.Parallel()
	m, err := LoadManifest("agy")
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	var rec recordedExec
	deps := Deps{EnvStage: "off", Runner: recordingRunner(&rec), Stderr: &stderr}

	if _, err := LaunchLLM(context.Background(), deps, LLMExec{Purpose: PurposeModelQuery, Binary: "agy", Args: []string{"models"}}); err != nil {
		t.Fatal(err)
	}

	want := "[bridge] modelquery agy runs unsandboxed: " + m.ProbeUnsandboxedReason
	if m.ProbeUnsandboxedReason == "" || !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestLaunchLLM_ABinaryWithNoManifestSaysSo(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	var rec recordedExec
	deps := Deps{EnvStage: "off", Runner: recordingRunner(&rec), Stderr: &stderr}

	if _, err := LaunchLLM(context.Background(), deps, LLMExec{Purpose: PurposeUpdate, Binary: "npm"}); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stderr.String(), "[bridge] update npm runs unsandboxed: "+noManifestReason) {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestLaunchLLM_WithNoRunnerOrStderrExecsTheBinary(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakellm")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho \"fakellm $1\"\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer

	rc, err := LaunchLLM(context.Background(), Deps{EnvStage: "off"}, LLMExec{Purpose: PurposeInventory, Binary: bin, Args: []string{"--version"}, Stdout: &out})

	if err != nil || rc != 3 || strings.TrimSpace(out.String()) != "fakellm --version" {
		t.Fatalf("rc=%d err=%v out=%q; want the exit code and the output of the real process", rc, err, out.String())
	}
}

func TestEveryManifestGivesTheReasonItsProbeRunsUnsandboxed(t *testing.T) {
	t.Parallel()
	for _, cli := range manifestCLIs(t) {
		m, err := LoadManifest(cli)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(m.ProbeUnsandboxedReason) == "" {
			t.Errorf("manifest %s has no probe_unsandboxed_reason", cli)
		}
	}
}

func TestModelQueryRunner_ReturnsTheCombinedOutputAndFailsOnANonZeroExit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bin := filepath.Join(dir, "fakeagy")
	script := "#!/bin/sh\nread line\necho \"out $1 $line\"\necho err >&2\n[ \"$1\" = models ] || exit 4\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := ModelQueryRunner(context.Background(), bin, []string{"models"}, "hi\n")
	if err != nil || out != "out models hi\nerr\n" {
		t.Fatalf("out=%q err=%v; want stdout and stderr together", out, err)
	}
	out, err = ModelQueryRunner(context.Background(), bin, []string{"--help"}, "")
	if err == nil || !strings.Contains(err.Error(), "exit 4") || !strings.Contains(out, "out --help") {
		t.Fatalf("out=%q err=%v; want the output and an exit 4 error", out, err)
	}
	if _, err := ModelQueryRunner(context.Background(), filepath.Join(dir, "absent"), nil, ""); err == nil {
		t.Fatal("a binary that cannot start must fail")
	}
}

type recordingPaneDriver struct {
	env    *[]string
	bypass *bool
}

func (recordingPaneDriver) Name() string { return "recording-tmux" }

func (d recordingPaneDriver) Launch(_ context.Context, cfg *Config, deps Deps) (int, error) {
	*d.env = deps.paneEnv(cfg)
	*d.bypass = cfg.AllowBypass
	return ExitOK, nil
}

func TestLaunchPane_TheSmokePurposeGrantsBypassAndKeepsCLAUDECODEFromThePane(t *testing.T) {
	for k, v := range pollutedEnv {
		t.Setenv(k, v)
	}
	for k, v := range runTmuxEnv {
		t.Setenv(k, v)
	}
	var env []string
	var bypass bool
	cfg := seatConfig("claude-tmux")

	rc, err := launchPane(context.Background(), PurposeSmoke, recordingPaneDriver{env: &env, bypass: &bypass}, cfg, Deps{EnvStage: "enforce"})

	if rc != ExitOK || err != nil || !bypass {
		t.Fatalf("rc=%d err=%v bypass=%v; want the launch API to grant bypass to a smoke pane", rc, err, bypass)
	}
	if names := envNames(env); env == nil || slices.Contains(names, "CLAUDECODE") {
		t.Errorf("smoke pane env names %v; want the cell env without CLAUDECODE", names)
	}
}

func TestLaunchPurposeConfig_GrantsBypassOnlyToTheProbeAndSmokePurposes(t *testing.T) {
	t.Parallel()
	for purpose, want := range map[LaunchPurpose]bool{PurposeProbe: true, PurposeSmoke: true, PurposeUpdate: false, PurposeInventory: false, PurposeModelQuery: false} {
		cfg := &Config{}
		launchPurposeConfig(purpose, cfg)
		if cfg.AllowBypass != want {
			t.Errorf("%s: AllowBypass = %v, want %v", purpose, cfg.AllowBypass, want)
		}
	}
}

func TestControllerFactory_TheUsageControllerTakesItsBypassFromTheProbePurpose(t *testing.T) {
	t.Parallel()
	f := NewControllerFactory("/repo", t.TempDir(), "", Deps{})
	c, ok := f.For("claude").(*cliController)
	if !ok || !c.cfg.AllowBypass || c.cfg.Agent != "control" {
		t.Fatalf("controller cfg = %+v; want the probe purpose bypass", c.cfg)
	}
}

func TestRecipeSession_ThePaneShellUnsetsTheRunTmuxBeforeTheLaunch(t *testing.T) {
	t.Parallel()
	fake := &fakeTmux{paneSeq: []string{"ready\n❯"}}
	var stderr bytes.Buffer
	deps := covDeps()
	deps.Tmux, deps.Stderr, deps.EnvStage = fake, &stderr, "off"
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Worktree: t.TempDir()}
	drv, _, err := newRecipeDriver(cfg, deps, "claude-tmux")
	if err != nil {
		t.Fatal(err)
	}

	_ = drv.EnsureSession(context.Background())

	detach := slices.Index(fake.sentKeys, paneTmuxDetachLine)
	launch := slices.Index(fake.sentKeys, drv.launchCmd)
	if detach < 0 || launch < 0 || detach > launch {
		t.Fatalf("sent %q; want %q before the launch command", fake.sentKeys, paneTmuxDetachLine)
	}
	if !strings.Contains(stderr.String(), "[bridge] probe claude runs unsandboxed: ") {
		t.Errorf("stderr = %q; want the manifest reason of the unsandboxed probe", stderr.String())
	}
}
