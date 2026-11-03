package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cloudlaunch"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	cloudCreatedFrame = "$ claude --cloud ...\nCreated cloud session: Fix the flaky test\nView: https://claude.ai/code/session_01AbC9?from=cli&m=0\nResume with: claude --teleport session_01AbC9\n$ "
	cloudTTYFrame     = "$ claude --cloud ...\nError: --cloud requires an interactive terminal.\n$ "
)

var pushedGitHubRepo = cloudlaunch.Repo{Branch: "feature/x", Remote: "origin", URL: "https://github.com/acme/widgets.git"}

type envCapturingTmux struct {
	*FakeTmuxController
	env []string
}

func (e *envCapturingTmux) NewSessionEnv(ctx context.Context, name string, width, height int, dir string, env []string) error {
	e.env = env
	return e.FakeTmuxController.NewSessionEnv(ctx, name, width, height, dir, env)
}

type cloudFixture struct {
	cfg     *Config
	deps    Deps
	tmux    *FakeTmuxController
	stderr  *bytes.Buffer
	signals *signalcenter.Center
	slept   time.Duration
}

func newCloudFixture(t *testing.T, repo cloudlaunch.Repo, frames ...string) *cloudFixture {
	t.Helper()
	ws := t.TempDir()
	prompt := filepath.Join(ws, "builder-prompt.txt")
	if err := os.WriteFile(prompt, []byte("Fix the flaky test. Token $CHALLENGE_TOKEN\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &cloudFixture{
		cfg: &Config{
			CLI: "claude-cloud", Profile: "builder", Agent: "builder", RunID: "run-7", Cycle: 3, Workspace: ws,
			PromptFile: prompt, Artifact: filepath.Join(ws, "builder-report.md"), Worktree: t.TempDir(),
			Realization: RealizeFor("claude-cloud", LaunchIntent{}),
		},
		tmux:    &FakeTmuxController{CaptureFrames: append(frames, "$ ")},
		stderr:  &bytes.Buffer{},
		signals: signalcenter.New(),
	}
	f.deps = Deps{
		Tmux: f.tmux, Stderr: f.stderr, Signals: f.signals, EnvStage: envStageOff, LookupEnv: mapLookup(nil),
		Now:               func() time.Time { return time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC) },
		NewChallengeToken: func() (string, error) { return "tok-42", nil },
		Sleep:             func(d time.Duration) { f.slept += d },
		CloudRepo:         func(context.Context, string) (cloudlaunch.Repo, error) { return repo, nil },
		CloudRemote:       func(context.Context, string, cloudlaunch.Repo) error { return nil },
	}
	return f
}

func (f *cloudFixture) launch(t *testing.T) (int, error) {
	t.Helper()
	return claudeCloudDriver{}.Launch(context.Background(), f.cfg, f.deps)
}

func (f *cloudFixture) codes() []signalcenter.Code {
	var out []signalcenter.Code
	for _, e := range f.signals.Recent() {
		if e.Code == CodeCloudLaunched || e.Code == CodeCloudLaunchRefused {
			out = append(out, e.Code)
		}
	}
	return out
}

func (f *cloudFixture) event(t *testing.T, code signalcenter.Code) signalcenter.Event {
	t.Helper()
	for _, e := range f.signals.Recent() {
		if e.Code == code {
			return e
		}
	}
	t.Fatalf("no %s signal in %+v", code, f.signals.Recent())
	return signalcenter.Event{}
}

func TestClaudeCloudDriver_IsARegisteredDriverOfTheClaudeFamily(t *testing.T) {
	t.Parallel()
	d, ok := LookupDriver("claude-cloud")
	if !ok || d.Name() != "claude-cloud" {
		t.Fatalf("LookupDriver(claude-cloud) = %v, %v; want the cloud driver", d, ok)
	}
	m, err := LoadManifest("claude-cloud")
	if err != nil {
		t.Fatalf("LoadManifest(claude-cloud) = %v", err)
	}
	if m.Binary != "claude" || m.ModelFamily != "claude" || m.Transport != "cloud" || m.IsTmux() {
		t.Errorf("manifest = binary %q family %q transport %q; want claude, claude, cloud", m.Binary, m.ModelFamily, m.Transport)
	}
	if m.CloudLaunch == nil || !slices.Equal(m.CloudLaunch.Args, []string{"--cloud"}) || !slices.Equal(m.CloudLaunch.RemoteHosts, []string{"github.com"}) {
		t.Errorf("cloud_launch = %+v, want --cloud on github.com", m.CloudLaunch)
	}
	if len(m.Controls) != 0 || len(m.ModelTierMap) != 0 || m.TranscriptFormat() != "" || m.HasCapability(CapabilityInprocessSubagents) {
		t.Errorf("the cloud seat declares a capability that it cannot serve: controls %v, tiers %v, transcript %q, capabilities %v", m.Controls, m.ModelTierMap, m.TranscriptFormat(), m.Capabilities)
	}
	base, _ := LoadManifest("claude-tmux")
	if !slices.Equal(m.EnvPassthrough, base.EnvPassthrough) {
		t.Errorf("env_passthrough = %v, want the claude-tmux allowlist %v", m.EnvPassthrough, base.EnvPassthrough)
	}
}

func TestClaudeCloudDriver_ThreeLinesGiveALaunchedSession(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, "$ ", "Created cloud session: Fix the flaky test\n", cloudCreatedFrame, "$ ")
	rc, err := f.launch(t)
	if rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v; stderr:\n%s", rc, err, f.stderr)
	}
	report, err := os.ReadFile(f.cfg.Artifact)
	if err != nil {
		t.Fatalf("no artifact: %v", err)
	}
	if !strings.HasPrefix(string(report), "<!-- challenge-token: tok-42 -->\n") {
		t.Errorf("the artifact does not carry the challenge token first:\n%s", report)
	}
	got, ok := cloudlaunch.Parse(string(report))
	if !ok {
		t.Fatalf("the artifact holds no launched session:\n%s", report)
	}
	want := cloudlaunch.Session{
		Schema: cloudlaunch.Schema, Outcome: cloudlaunch.OutcomeLaunched, ID: "session_01AbC9",
		URL: "https://claude.ai/code/session_01AbC9?from=cli&m=0", Title: "Fix the flaky test",
		Repo: "acme/widgets", Branch: "feature/x", Profile: "builder", CLI: "claude-cloud", TS: "2026-11-03T10:00:00Z",
	}
	if got != want {
		t.Errorf("session = %+v, want %+v", got, want)
	}
	if f.slept != 3*time.Second {
		t.Errorf("the launch waited %s, want 3s: it stops at the first frame with both lines", f.slept)
	}
}

func TestClaudeCloudDriver_TypesTheCloudCommandInItsOwnSessionAndEndsOnlyThatSession(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	f.tmux.Existing = map[string]bool{"evolve-bridge-other": true}
	if rc, err := f.launch(t); rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v", rc, err)
	}
	var own string
	for _, ev := range f.tmux.Events {
		if name, ok := strings.CutPrefix(ev, "new-session:"); ok {
			own = name
		}
	}
	if !strings.HasPrefix(own, "evolve-cloud-") {
		t.Fatalf("the launch started session %q, want a private evolve-cloud- session; events %v", own, f.tmux.Events)
	}
	if !slices.Equal(f.tmux.KilledSessions, []string{own}) {
		t.Errorf("killed sessions = %v, want only the own session %s", f.tmux.KilledSessions, own)
	}
	if !f.tmux.Existing["evolve-bridge-other"] {
		t.Error("the launch ended a session that it did not start")
	}
	promptFile := filepath.Join(f.cfg.Workspace, "cloud-prompt.txt")
	wantCmd := `claude --cloud -- "$(cat ` + promptFile + `)"`
	if !slices.Contains(f.tmux.SentKeys, wantCmd) {
		t.Errorf("sent keys %q lack the launch %q", f.tmux.SentKeys, wantCmd)
	}
	if !slices.Contains(f.tmux.SentKeys, "cd "+f.cfg.Worktree) {
		t.Errorf("the pane did not change to the worktree %s: %q", f.cfg.Worktree, f.tmux.SentKeys)
	}
	prompt, _ := os.ReadFile(promptFile)
	if string(prompt) != "Fix the flaky test. Token "+cloudWithheldToken+"\n" {
		t.Errorf("cloud prompt = %q, want the resolved prompt with the token withheld", prompt)
	}
}

func TestClaudeCloudDriver_RecordsTheLaunchInOneSignal(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	if rc, err := f.launch(t); rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v", rc, err)
	}
	if got := f.codes(); !slices.Equal(got, []signalcenter.Code{CodeCloudLaunched}) {
		t.Fatalf("signals = %v, want one %s", got, CodeCloudLaunched)
	}
	e := f.event(t, CodeCloudLaunched)
	want := map[string]string{"cli": "claude-cloud", "session_id": "session_01AbC9", "url": "https://claude.ai/code/session_01AbC9?from=cli&m=0", "repo": "acme/widgets", "branch": "feature/x", "profile": "builder"}
	for k, v := range want {
		if e.Fields[k] != v {
			t.Errorf("field %s = %q, want %q", k, e.Fields[k], v)
		}
	}
	if e.Cycle != 3 || e.RunID != "run-7" || e.Phase != "builder" || e.Severity != signalcenter.SeverityInfo {
		t.Errorf("event identity = %+v", e)
	}
}

func TestClaudeCloudDriver_ADeadlineWithNoCreatedLineFailsWithoutAFallbackExit(t *testing.T) {
	t.Parallel()
	frames := make([]string, 0, 120)
	for range 120 {
		frames = append(frames, "$ claude --cloud ...\nConnecting...\n")
	}
	f := newCloudFixture(t, pushedGitHubRepo, frames...)
	rc, err := f.launch(t)
	if rc != ExitSafetyGate || err != nil {
		t.Fatalf("Launch = %d, %v; want exit %d", rc, err, ExitSafetyGate)
	}
	if !strings.Contains(f.stderr.String(), "no session confirmation within 120s") {
		t.Errorf("stderr lacks the deadline reason:\n%s", f.stderr)
	}
	if f.slept != 120*time.Second {
		t.Errorf("waited %v, want the 120s deadline in full", f.slept)
	}
	if _, err := os.Stat(f.cfg.Artifact); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a launch with no confirmation wrote an artifact: %v", err)
	}
	if len(f.tmux.KilledSessions) != 1 || len(f.codes()) != 0 {
		t.Errorf("killed %v, signals %v; want the own session killed and no launch signal", f.tmux.KilledSessions, f.codes())
	}
}

func TestClaudeCloudDriver_TheNotATTYErrorIsTheFailureReason(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudTTYFrame)
	rc, err := f.launch(t)
	if rc != ExitBadFlags || err != nil {
		t.Fatalf("Launch = %d, %v; want exit %d", rc, err, ExitBadFlags)
	}
	if !strings.Contains(f.stderr.String(), "--cloud requires an interactive terminal.") {
		t.Errorf("stderr lacks the CLI error text:\n%s", f.stderr)
	}
}

func TestClaudeCloudDriver_ACancelledLaunchStopsWaiting(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rc, err := claudeCloudDriver{}.Launch(ctx, f.cfg, f.deps)
	if rc != ExitSafetyGate || !errors.Is(err, context.Canceled) {
		t.Errorf("Launch = %d, %v; want exit %d and the cancel", rc, err, ExitSafetyGate)
	}
}

func TestClaudeCloudDriver_RefusesAnUnpushedBranchOrAnotherHostBeforeAnySession(t *testing.T) {
	t.Parallel()
	ahead := pushedGitHubRepo
	ahead.Ahead = 2
	gitlab := pushedGitHubRepo
	gitlab.URL = "https://gitlab.com/acme/widgets.git"
	for name, tc := range map[string]struct {
		repo cloudlaunch.Repo
		want string
	}{
		"unpushed":   {ahead, "2 commit(s) that are not pushed"},
		"non-GitHub": {gitlab, "not on a host of the manifest (github.com)"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newCloudFixture(t, tc.repo)
			rc, err := f.launch(t)
			if rc != ExitSafetyGate || err != nil {
				t.Fatalf("Launch = %d, %v; want exit %d", rc, err, ExitSafetyGate)
			}
			if !strings.Contains(f.stderr.String(), tc.want) {
				t.Errorf("stderr lacks %q:\n%s", tc.want, f.stderr)
			}
			if len(f.tmux.Events) != 0 {
				t.Errorf("a refused launch touched tmux: %v", f.tmux.Events)
			}
			e := f.event(t, CodeCloudLaunchRefused)
			if !strings.Contains(e.Reason, tc.want) || e.Severity != signalcenter.SeverityWarn || e.Fields["cli"] != "claude-cloud" {
				t.Errorf("refusal signal = %+v", e)
			}
			if got := f.codes(); !slices.Equal(got, []signalcenter.Code{CodeCloudLaunchRefused}) {
				t.Errorf("signals = %v, want one refusal", got)
			}
		})
	}
}

func TestClaudeCloudDriver_ReadsTheRealRepositoryWhenNoReaderIsInjected(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	f.deps.CloudRepo = nil
	rc, err := f.launch(t)
	if rc != ExitSafetyGate || err != nil || !strings.Contains(f.stderr.String(), "on no branch") {
		t.Errorf("Launch in a tree with no git = %d, %v; stderr:\n%s", rc, err, f.stderr)
	}
}

func TestClaudeCloudDriver_TheLaunchEnvironmentFollowsTheManifestAllowlist(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "oauth-ok")
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	capture := &envCapturingTmux{FakeTmuxController: f.tmux}
	f.deps.Tmux = capture
	f.deps.EnvStage = envStageEnforce
	if rc, err := f.launch(t); rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v; stderr:\n%s", rc, err, f.stderr)
	}
	if !slices.Contains(capture.env, "CLAUDE_CODE_OAUTH_TOKEN=oauth-ok") {
		t.Errorf("the pane env lacks the sign-in token of the allowlist: %v", capture.env)
	}
	for _, kv := range capture.env {
		if strings.HasPrefix(kv, "GITHUB_TOKEN=") {
			t.Errorf("the pane env inherits %s, which is outside the allowlist", kv)
		}
	}
}

func TestClaudeCloudDriver_KeepsTheCredentialGateOfTheClaudeFamily(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	f.deps.LookupEnv = mapLookup(map[string]string{"ANTHROPIC_API_KEY": "sk"})
	if rc, _ := f.launch(t); rc != ExitCostLeak {
		t.Errorf("Launch with ANTHROPIC_API_KEY = %d, want %d", rc, ExitCostLeak)
	}
	if !strings.Contains(f.stderr.String(), "[claude-cloud] ANTHROPIC_API_KEY is set") {
		t.Errorf("stderr = %q, want the cloud prefix", f.stderr)
	}
}

func TestLaunchCloud_RefusesAManifestWithNoUsableCloudLaunch(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	if rc, err := launchCloud(context.Background(), f.cfg, f.deps, Manifest{CLI: "x", Binary: "x"}); rc != ExitBadFlags || err == nil {
		t.Errorf("launchCloud with no cloud_launch = %d, %v; want a refusal", rc, err)
	}
	bad := Manifest{CLI: "x", Binary: "x", CloudLaunch: &CloudLaunchSpec{Args: []string{"--cloud"}, SessionRegex: "(", DeadlineS: 1, RemoteHosts: []string{"github.com"}}}
	if rc, err := launchCloud(context.Background(), f.cfg, f.deps, bad); rc != ExitBadFlags || err == nil {
		t.Errorf("launchCloud with a bad pattern = %d, %v; want a refusal", rc, err)
	}
}

func TestLaunchCloud_FailsWhenTheSessionOrTheWorkspaceCannotBeMade(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	f.tmux.NewSessionErr = errors.New("no server")
	if rc, err := f.launch(t); rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "no server") {
		t.Errorf("Launch with no tmux = %d, %v", rc, err)
	}
	g := newCloudFixture(t, pushedGitHubRepo)
	g.deps.EnvStage = envStageEnforce
	g.deps.Tmux = sessionOnlyTmux{g.tmux}
	if rc, err := g.launch(t); rc != ExitBadFlags || !errors.Is(err, errNoEnvSessionStart) {
		t.Errorf("Launch with a controller that cannot carry the env = %d, %v", rc, err)
	}
	h := newCloudFixture(t, pushedGitHubRepo)
	h.cfg.Workspace = filepath.Join(h.cfg.PromptFile, "under-a-file")
	if rc, err := h.launch(t); rc != ExitBadFlags || err == nil {
		t.Errorf("Launch with a workspace under a file = %d, %v", rc, err)
	}
}

func TestLaunchCloud_FailsOnAPromptOrAnArtifactItCannotWrite(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	f.cfg.PromptFile = filepath.Join(f.cfg.Workspace, "absent.txt")
	if rc, err := f.launch(t); rc != ExitBadFlags || err == nil {
		t.Errorf("Launch with no prompt = %d, %v", rc, err)
	}
	g := newCloudFixture(t, pushedGitHubRepo)
	if err := os.Mkdir(filepath.Join(g.cfg.Workspace, "cloud-prompt.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if rc, err := g.launch(t); rc != ExitBadFlags || err == nil {
		t.Errorf("Launch with an unwritable cloud prompt = %d, %v", rc, err)
	}
	h := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	if os.Geteuid() == 0 {
		t.Skip("a read-only directory does not stop root")
	}
	h.deps.Sleep = func(time.Duration) { _ = os.Chmod(h.cfg.Workspace, 0o500) }
	t.Cleanup(func() { _ = os.Chmod(h.cfg.Workspace, 0o700) })
	if rc, err := h.launch(t); rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "session_01AbC9") {
		t.Errorf("Launch with an unwritable artifact = %d, %v; want an error that names the launched session", rc, err)
	}
	if got := h.codes(); !slices.Equal(got, []signalcenter.Code{CodeCloudLaunched}) {
		t.Errorf("signals = %v; a launched session is signalled before its report is written", got)
	}
}

func TestLaunchCloud_AnArtifactThatCannotBeClearedStopsTheLaunchBeforeTmux(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	if err := os.MkdirAll(filepath.Join(f.cfg.Artifact, "held"), 0o755); err != nil {
		t.Fatal(err)
	}
	rc, err := f.launch(t)
	if rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "clear the artifact") {
		t.Errorf("Launch = %d, %v; want a refusal that names the artifact", rc, err)
	}
	if len(f.tmux.Events) != 0 {
		t.Errorf("a launch that could not clear the artifact touched tmux: %v", f.tmux.Events)
	}
}

func TestLaunchCloud_RefusesAPromptOverTheArgumentLimit(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	if err := os.WriteFile(f.cfg.PromptFile, bytes.Repeat([]byte("x"), cloudMaxPromptBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, err := f.launch(t)
	if rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "argument limit") {
		t.Errorf("Launch with an oversized prompt = %d, %v; want a refusal that names the limit", rc, err)
	}
	if len(f.tmux.Events) != 0 {
		t.Errorf("an oversized prompt touched tmux: %v", f.tmux.Events)
	}
}

func TestLaunchCloud_ALostPaneEndsTheWaitAtOnce(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	f.deps.Tmux = &lostPaneTmux{FakeTmuxController: f.tmux}
	rc, err := f.launch(t)
	if rc != ExitSafetyGate || err != nil {
		t.Fatalf("Launch with a lost pane = %d, %v; want exit %d", rc, err, ExitSafetyGate)
	}
	if f.slept != time.Second || !strings.Contains(f.stderr.String(), "the pane of session") {
		t.Errorf("waited %s, stderr %q; want one tick and the lost pane named", f.slept, f.stderr)
	}
}

func TestLaunchesCloud_OnlyAManifestWithACloudLaunch(t *testing.T) {
	t.Parallel()
	for cli, want := range map[string]bool{"claude-cloud": true, "claude-tmux": false, "claude-p": false, "absent": false} {
		if got := LaunchesCloud(cli); got != want {
			t.Errorf("LaunchesCloud(%q) = %v, want %v", cli, got, want)
		}
	}
}

type lostPaneTmux struct{ *FakeTmuxController }

func (l *lostPaneTmux) CapturePane(ctx context.Context, session string, _ int) (string, error) {
	_ = l.FakeTmuxController.KillSession(ctx, session)
	return "", errors.New("no such session")
}

func TestLaunchCloud_ASendFailureEndsTheOwnSession(t *testing.T) {
	t.Parallel()
	for _, failOn := range []string{"--cloud", paneTmuxDetachLine} {
		t.Run(failOn, func(t *testing.T) {
			f := newCloudFixture(t, pushedGitHubRepo)
			f.deps.Tmux = &sendFailTmux{FakeTmuxController: f.tmux, failOn: failOn}
			rc, err := f.launch(t)
			if rc != ExitBadFlags || err == nil {
				t.Errorf("Launch with a failing send = %d, %v", rc, err)
			}
			if len(f.tmux.KilledSessions) != 1 {
				t.Errorf("killed %v, want the own session", f.tmux.KilledSessions)
			}
		})
	}
}

func TestLaunchCloud_ACaptureErrorIsRetriedUntilTheLines(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	flaky := &flakyCaptureTmux{FakeTmuxController: f.tmux, fails: 2}
	f.tmux.CaptureFrames = []string{cloudCreatedFrame, "$ "}
	f.deps.Tmux = flaky
	if rc, err := f.launch(t); rc != ExitOK || err != nil {
		t.Errorf("Launch = %d, %v", rc, err)
	}
}

func TestLaunchCloud_ThePromptGoesFromTheWorkspaceWhenThereIsNoWorktree(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	f.cfg.Worktree = ""
	f.cfg.ProjectRoot = t.TempDir()
	var dir string
	f.deps.CloudRepo = func(_ context.Context, d string) (cloudlaunch.Repo, error) { dir = d; return pushedGitHubRepo, nil }
	if rc, err := f.launch(t); rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v", rc, err)
	}
	if dir != f.cfg.ProjectRoot {
		t.Errorf("the repository was read in %q, want the project root %q", dir, f.cfg.ProjectRoot)
	}
	f.cfg.ProjectRoot = ""
	cwd, _ := os.Getwd()
	if got := cloudWorkingDir(f.cfg); got != cwd {
		t.Errorf("cloudWorkingDir with no worktree and no root = %q, want the cwd %q", got, cwd)
	}
}

func TestEngineLaunch_ClaudeCloudDispatchRecordsTheLaunchedRow(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "cloud-test", "")
	artifact := filepath.Join(ws, "builder-report.md")
	tmux := &FakeTmuxController{CaptureFrames: []string{cloudCreatedFrame, "$ ", "$ "}}
	eng := NewEngine(Deps{
		Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil), EnvStage: envStageOff,
		CloudRepo:   func(context.Context, string) (cloudlaunch.Repo, error) { return pushedGitHubRepo, nil },
		CloudRemote: func(context.Context, string, cloudlaunch.Repo) error { return nil },
	})
	resp, err := eng.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-cloud", Profile: prof, Model: "sonnet", Prompt: "Fix the flaky test", Workspace: ws,
		ArtifactPath: artifact, Agent: "builder", Worktree: t.TempDir(),
	})
	if err != nil || resp.ExitCode != ExitOK {
		t.Fatalf("Launch = %+v, %v", resp, err)
	}
	s, ok := cloudlaunch.Parse(resp.Stdout)
	if !ok || s.ID != "session_01AbC9" {
		t.Errorf("the response carries no launched session: %q", resp.Stdout)
	}
	launch := slices.IndexFunc(tmux.SentKeys, func(k string) bool { return strings.Contains(k, "--cloud") })
	if launch < 0 || !strings.HasPrefix(tmux.SentKeys[launch], `claude --cloud -- "$(cat `) {
		t.Errorf("sent keys = %q, want the bare cloud launch with no local flag", tmux.SentKeys)
	}
	read, err := llmcalls.ReadWorkspace(ws)
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("llm-calls rows = %d, %v; want one", len(read.Records), err)
	}
	if row := read.Records[0]; row.CLI != "claude-cloud" || row.Agent != "builder" {
		t.Errorf("llm-calls row = cli %q agent %q", row.CLI, row.Agent)
	}
}

type sessionOnlyTmux struct{ *FakeTmuxController }

func (s sessionOnlyTmux) NewSessionEnv() {}

type sendFailTmux struct {
	*FakeTmuxController
	failOn string
}

func (s *sendFailTmux) SendKeys(_ context.Context, _ string, keys string, _ bool) error {
	if strings.Contains(keys, s.failOn) {
		return errors.New("send failed")
	}
	return nil
}

type flakyCaptureTmux struct {
	*FakeTmuxController
	fails int
}

func (f *flakyCaptureTmux) CapturePane(ctx context.Context, session string, n int) (string, error) {
	if f.fails > 0 {
		f.fails--
		return "", errors.New("capture failed")
	}
	return f.FakeTmuxController.CapturePane(ctx, session, n)
}

func TestClaudeCloudDriver_AManifestThatDoesNotLoadRefusesTheLaunch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "claude-cloud.json"), []byte(`{"cli":"claude-cloud","binary":"claude","cloud_launch":{"args":[],"deadline_s":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	useBridgeManifestDir(t, dir)
	f := newCloudFixture(t, pushedGitHubRepo)
	if rc, err := f.launch(t); rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "cloud_launch") {
		t.Errorf("Launch with a bad manifest = %d, %v; want a refusal that names cloud_launch", rc, err)
	}
}

func TestLaunchCloud_ARepositoryReadErrorThatIsNoRefusalIsABadFlag(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo)
	f.deps.CloudRepo = func(context.Context, string) (cloudlaunch.Repo, error) {
		return cloudlaunch.Repo{}, errors.New("git missing")
	}
	if rc, err := f.launch(t); rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "git missing") {
		t.Errorf("Launch with a repository read error = %d, %v", rc, err)
	}
	if len(f.codes()) != 0 {
		t.Errorf("signals = %v; a read error is no refusal", f.codes())
	}
}

func TestClaudeCloudManifest_InheritsOnlyTheKeysTheSeatServes(t *testing.T) {
	t.Parallel()
	raw, err := resolvedManifestBytes("claude-cloud")
	if err != nil {
		t.Fatal(err)
	}
	var resolved map[string]json.RawMessage
	if err := json.Unmarshal(raw, &resolved); err != nil {
		t.Fatal(err)
	}
	allowed := []string{
		"schema_version", "cli", "binary", "probe_unsandboxed_reason", "install_roots", "env_passthrough", "model_family",
		"capabilities", "home_reads", "context_window", "binary_min_version", "transport", "default_tier", "tier_dependencies",
		"default_args", "interactive_prompts", "note", "cloud_launch",
	}
	for key := range resolved {
		if !slices.Contains(allowed, key) {
			t.Errorf("claude-cloud inherits %q from its base; remove it in the manifest or add it here when the seat serves it", key)
		}
	}
}

type joinedTmux struct {
	*FakeTmuxController
	joined, plain int
}

func (j *joinedTmux) CapturePane(ctx context.Context, session string, n int) (string, error) {
	j.plain++
	return j.FakeTmuxController.CapturePane(ctx, session, n)
}

func (j *joinedTmux) CapturePaneJoined(ctx context.Context, session string, n int) (string, error) {
	j.joined++
	return j.FakeTmuxController.CapturePane(ctx, session, n)
}

func TestClaudeCloudDriver_ReadsTheConfirmationFromTheJoinedPaneWhenTheControllerOffersIt(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	j := &joinedTmux{FakeTmuxController: f.tmux}
	f.deps.Tmux = j
	rc, err := f.launch(t)
	if rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v; want a launch", rc, err)
	}
	if j.joined == 0 {
		t.Error("the wait read no joined pane: a wrapped title row could forge the view line")
	}
	if j.plain > 1 {
		t.Errorf("the wait read %d plain panes; only the cleanup capture may be plain", j.plain)
	}
}

func TestClaudeCloudDriver_ClearsTheArtifactOfAnEarlierSeatBeforeItStarts(t *testing.T) {
	t.Parallel()
	ahead := pushedGitHubRepo
	ahead.Ahead = 1
	for name, f := range map[string]*cloudFixture{
		"a refusal":  newCloudFixture(t, ahead),
		"a deadline": newCloudFixture(t, pushedGitHubRepo),
	} {
		t.Run(name, func(t *testing.T) {
			stale := cloudlaunch.Render(cloudlaunch.Session{Schema: cloudlaunch.Schema, Outcome: cloudlaunch.OutcomeLaunched, ID: "session_FORGED"}, "")
			if err := os.WriteFile(f.cfg.Artifact, []byte(stale), 0o644); err != nil {
				t.Fatal(err)
			}
			f.tmux.CaptureFrames = append(make([]string, 0, 130), slices.Repeat([]string{"Connecting...\n"}, 125)...)
			if _, err := f.launch(t); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(f.cfg.Artifact); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the artifact of an earlier seat survived a failed cloud launch: %v", err)
			}
		})
	}
}

func TestClaudeCloudDriver_TheCloudPromptHoldsNoChallengeTokenAndNoAbsolutePath(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	if err := os.WriteFile(f.cfg.PromptFile, []byte("Line one: <!-- challenge-token: $CHALLENGE_TOKEN -->\nWrite $ARTIFACT_PATH.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, err := f.launch(t); rc != ExitOK || err != nil {
		t.Fatalf("Launch = %d, %v; stderr:\n%s", rc, err, f.stderr)
	}
	raw, err := os.ReadFile(filepath.Join(f.cfg.Workspace, cloudPromptFile))
	if err != nil {
		t.Fatal(err)
	}
	prompt := string(raw)
	want := "Line one: <!-- challenge-token: " + cloudWithheldToken + " -->\nWrite builder-report.md.\n"
	if prompt != want {
		t.Errorf("cloud prompt = %q, want %q", prompt, want)
	}
	for _, leak := range []string{"tok-42", f.cfg.Workspace, f.cfg.Worktree, string(filepath.Separator) + "builder-report.md"} {
		if strings.Contains(prompt, leak) {
			t.Errorf("the cloud prompt holds %q:\n%s", leak, prompt)
		}
	}
	report, _ := os.ReadFile(f.cfg.Artifact)
	if !strings.HasPrefix(string(report), "<!-- challenge-token: tok-42 -->\n") {
		t.Errorf("the local report lost the challenge token:\n%s", report)
	}
}

func TestClaudeCloudDriver_ARemoteHeadThatDiffersFromTheLocalHeadRefusesTheLaunch(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	var checked cloudlaunch.Repo
	var dir string
	f.deps.CloudRemote = func(_ context.Context, d string, r cloudlaunch.Repo) error {
		checked, dir = r, d
		return &cloudlaunch.Refusal{Reason: "the head of feature/x on origin differs from the local head"}
	}
	rc, err := f.launch(t)
	if rc != ExitSafetyGate || err != nil {
		t.Fatalf("Launch = %d, %v; want exit %d", rc, err, ExitSafetyGate)
	}
	if checked != pushedGitHubRepo || dir != f.cfg.Worktree {
		t.Errorf("the remote check read %+v in %q, want the read repository in the worktree", checked, dir)
	}
	if len(f.tmux.Events) != 0 {
		t.Errorf("a refused launch touched tmux: %v", f.tmux.Events)
	}
	if e := f.event(t, CodeCloudLaunchRefused); !strings.Contains(e.Reason, "differs from the local head") {
		t.Errorf("refusal signal = %+v", e)
	}
	if !strings.Contains(f.stderr.String(), "launch refused: the head of feature/x") {
		t.Errorf("stderr lacks the refusal:\n%s", f.stderr)
	}
}

func TestClaudeCloudDriver_ARemoteReadErrorThatIsNoRefusalIsABadFlag(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	f.deps.CloudRemote = func(context.Context, string, cloudlaunch.Repo) error { return errors.New("network down") }
	rc, err := f.launch(t)
	if rc != ExitBadFlags || err == nil || !strings.Contains(err.Error(), "read the remote head of branch feature/x: network down") {
		t.Errorf("Launch = %d, %v; want exit %d and the read error", rc, err, ExitBadFlags)
	}
	if len(f.tmux.Events) != 0 || len(f.codes()) != 0 {
		t.Errorf("tmux %v, signals %v; want none", f.tmux.Events, f.codes())
	}
}

func TestClaudeCloudDriver_ReadsTheRealRemoteHeadWhenNoReaderIsInjected(t *testing.T) {
	t.Parallel()
	f := newCloudFixture(t, pushedGitHubRepo, cloudCreatedFrame)
	f.deps.CloudRemote = nil
	rc, err := f.launch(t)
	if rc != ExitSafetyGate || err != nil || !strings.Contains(f.stderr.String(), "has no local head") {
		t.Errorf("Launch in a tree with no git = %d, %v; stderr:\n%s", rc, err, f.stderr)
	}
	if len(f.tmux.Events) != 0 {
		t.Errorf("a refused launch touched tmux: %v", f.tmux.Events)
	}
}
