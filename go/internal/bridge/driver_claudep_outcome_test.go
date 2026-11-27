package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
)

func claudePFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "claude-p-results", name))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	return string(data)
}

type claudePRun struct {
	rc     int
	stderr string
	cfg    *Config
	args   []string
}

func launchClaudeP(t *testing.T, cfg *Config, deps Deps, runner CmdRunner) claudePRun {
	t.Helper()
	ws := t.TempDir()
	if cfg.Workspace == "" {
		cfg.Workspace = ws
	}
	cfg.PromptFile = filepath.Join(ws, "prompt.txt")
	if err := os.WriteFile(cfg.PromptFile, []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.StdoutLog = filepath.Join(ws, "stdout.log")
	cfg.StderrLog = filepath.Join(ws, "stderr.log")
	cfg.Artifact = filepath.Join(ws, "artifact.md")
	cfg.Worktree = ws
	cfg.ProjectRoot = ws
	cfg.CLI = "claude-p"
	var stderr bytes.Buffer
	var args []string
	deps.Stderr = &stderr
	deps.Env = map[string]string{}
	deps.Runner = func(ctx context.Context, name, dir string, a, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		args = a
		return runner(ctx, name, dir, a, env, in, out, errw)
	}
	rc, err := claudePDriver{}.Launch(context.Background(), cfg, deps.withDefaults())
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	return claudePRun{rc: rc, stderr: stderr.String(), cfg: cfg, args: args}
}

func streamRunner(stdout, stderr string, rc int) CmdRunner {
	return func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, errw io.Writer) (int, error) {
		_, _ = io.WriteString(out, stdout)
		_, _ = io.WriteString(errw, stderr)
		return rc, nil
	}
}

func TestClaudePLaunch_ResultClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, fixture, stderr string
		rc                    int
		wantRC                int
		wantCause             string
		quota, login          bool
	}{
		{"usage limit result exits 85 as rate_limit", "usage-limit.jsonl", "", 1, ExitUnknownPrompt, "rate_limit", true, false},
		{"logged-out result exits 85 as the login cause", "auth-error.jsonl", "", 1, ExitUnknownPrompt, clihealth.CredentialPattern, false, true},
		{"generic error stays a plain failure", "generic-error.jsonl", "", 1, 1, "", false, false},
		{"clean success stays success even if the text names a limit", "success.jsonl", "", 0, 0, "", false, false},
		{"a non-error result that names a limit does not escalate on a failing exit", "success.jsonl", "", 1, 1, "", false, false},
		{"stderr usage limit with no result event", "", "Claude AI usage limit reached|1700000000\n", 1, ExitUnknownPrompt, "rate_limit", true, false},
		{"stderr login wall with no result event", "", "Not logged in · Please run /login\n", 1, ExitUnknownPrompt, clihealth.CredentialPattern, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stdout := ""
			if tc.fixture != "" {
				stdout = claudePFixture(t, tc.fixture)
			}
			run := launchClaudeP(t, &Config{}, Deps{}, streamRunner(stdout, tc.stderr, tc.rc))
			if run.rc != tc.wantRC {
				t.Fatalf("exit = %d, want %d; stderr=%s", run.rc, tc.wantRC, run.stderr)
			}
			out := launchoutcome.Classify(run.rc, nil, run.stderr)
			if out.CauseCode == tc.wantCause && tc.wantCause != "" {
				if got := out.Signal == launchoutcome.CodeExitQuotaWall; got != tc.quota {
					t.Fatalf("quota signal = %v, want %v (cause %q)", got, tc.quota, out.CauseCode)
				}
				if got := clihealth.LoginPattern(out.CauseCode); got != tc.login {
					t.Fatalf("login = %v, want %v", got, tc.login)
				}
				return
			}
			if tc.wantCause != "" || out.Signal == launchoutcome.CodeExitQuotaWall {
				t.Fatalf("cause = %q signal = %v, want cause %q", out.CauseCode, out.Signal, tc.wantCause)
			}
		})
	}
}

func TestClaudePLaunch_PlainStdoutErrorClassifies(t *testing.T) {
	t.Parallel()
	run := launchClaudeP(t, &Config{}, Deps{}, streamRunner("working...\nClaude AI usage limit reached|1700000000\n", "", 1))
	if run.rc != ExitUnknownPrompt {
		t.Fatalf("exit = %d, want %d", run.rc, ExitUnknownPrompt)
	}
}

func TestClaudePLaunch_PlainStdoutNoiseDoesNotEscalate(t *testing.T) {
	t.Parallel()
	body := "the docs say usage limit reached means wait\nthen it crashed\n"
	run := launchClaudeP(t, &Config{}, Deps{}, streamRunner(body, "", 1))
	if run.rc != 1 {
		t.Fatalf("exit = %d, want 1", run.rc)
	}
}

func TestClaudePLaunch_UnreadableManifestKeepsTheExit(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	if got := classifyHeadlessExit(1, "no-such-cli", &Config{}, Deps{Stderr: &stderr}); got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "manifest unavailable") {
		t.Fatalf("stderr = %q, want the manifest warning", stderr.String())
	}
}

func TestClaudePLaunch_BadRulesKeepTheExit(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	spec := &HeadlessOutcomeSpec{Rules: []HeadlessRule{{Pattern: "rate_limit", Source: "result", Regex: "("}}}
	if got := classifyWithSpec(1, spec, &Config{}, Deps{Stderr: &stderr}); got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "rules unusable") {
		t.Fatalf("stderr = %q, want the rules warning", stderr.String())
	}
}

func TestClaudePLaunch_IdleTimeoutExits81(t *testing.T) {
	t.Parallel()
	clock := newTestClock()
	asked := make(chan time.Duration)
	ticks := make(chan time.Time)
	deps := Deps{After: func(d time.Duration) <-chan time.Time { asked <- d; return ticks }, ActivityNow: clock.Now}
	started := make(chan struct{})
	runner := func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		close(started)
		<-ctx.Done()
		return -1, ctx.Err()
	}
	go func() {
		<-started
		if got := <-asked; got != 49*time.Second {
			t.Errorf("idle wait = %v, want the full budget 49s", got)
		}
		clock.Advance(49 * time.Second)
		ticks <- clock.Now()
	}()
	run := launchClaudeP(t, &Config{StreamOutput: true, ArtifactTimeoutS: 7}, deps, runner)
	if run.rc != ExitArtifactTimeout {
		t.Fatalf("exit = %d, want %d; stderr=%s", run.rc, ExitArtifactTimeout, run.stderr)
	}
	if !strings.Contains(run.stderr, launchoutcome.IdleOutputLine(49)) {
		t.Fatalf("stderr = %q, want the typed idle line", run.stderr)
	}
	if out := launchoutcome.Classify(run.rc, nil, run.stderr); out.CauseCode != string(launchoutcome.TimeoutIdleOutput) {
		t.Fatalf("cause = %q, want %s", out.CauseCode, launchoutcome.TimeoutIdleOutput)
	}
}

func TestClaudePLaunch_NonStreamRunIsNeverIdleWatched(t *testing.T) {
	t.Parallel()
	deps := Deps{After: func(time.Duration) <-chan time.Time {
		t.Error("a non-stream run must not arm the idle watch")
		return nil
	}}
	run := launchClaudeP(t, &Config{ArtifactTimeoutS: 7}, deps, streamRunner("", "", 0))
	if run.rc != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", run.rc, run.stderr)
	}
}

func TestClaudePLaunch_GrowingOutputKeepsAStreamRunAlive(t *testing.T) {
	t.Parallel()
	clock := newTestClock()
	asked := make(chan time.Duration)
	ticks := make(chan time.Time)
	grow := make(chan struct{})
	wrote := make(chan struct{})
	release := make(chan struct{})
	deps := Deps{After: func(d time.Duration) <-chan time.Time { asked <- d; return ticks }, ActivityNow: clock.Now}
	runner := func(_ context.Context, _, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		for {
			select {
			case <-grow:
				_, _ = io.WriteString(out, "{\"type\":\"stream_event\"}\n")
				wrote <- struct{}{}
			case <-release:
				return 0, nil
			}
		}
	}
	go func() {
		for i := 0; i < 3; i++ {
			if got := <-asked; got != 49*time.Second {
				t.Errorf("round %d wait = %v, want 49s", i, got)
			}
			grow <- struct{}{}
			<-wrote
			clock.Advance(100 * time.Second)
			ticks <- clock.Now()
		}
		<-asked
		close(release)
	}()
	run := launchClaudeP(t, &Config{StreamOutput: true, ArtifactTimeoutS: 7}, deps, runner)
	if run.rc != 0 {
		t.Fatalf("exit = %d, want 0 (a writing run is never idle); stderr=%s", run.rc, run.stderr)
	}
}

func TestClaudePLaunch_CrashDuringModelTextIsNotQuota(t *testing.T) {
	t.Parallel()
	stdout := "{\"type\":\"stream_event\",\"event\":{\"delta\":{\"text\":\"the classifier maps rate_limit_error to exit 85\"}}}\n"
	run := launchClaudeP(t, &Config{}, Deps{}, streamRunner(stdout, "Error: process crashed\n", 1))
	if run.rc != 1 {
		t.Fatalf("exit = %d, want the plain failure 1; stderr=%s", run.rc, run.stderr)
	}
}

func TestClaudePLaunch_StderrFreeTextIsNotQuota(t *testing.T) {
	t.Parallel()
	tail := "Claude AI usage limit reached|1700000000\nl2\nl3\nl4\nl5\nl6\n"
	for _, tc := range []struct{ name, fixture, stderr string }{
		{"percent left in free text", "generic-error.jsonl", "tip: 0% left means the budget is gone\n"},
		{"too many requests in free text", "generic-error.jsonl", "tip: too many requests is a 429\n"},
		{"a limit line above the last lines", "generic-error.jsonl", tail},
		{"a clean result with a limit line on stderr", "success.jsonl", "Claude AI usage limit reached|1700000000\n"},
	} {
		run := launchClaudeP(t, &Config{}, Deps{}, streamRunner(claudePFixture(t, tc.fixture), tc.stderr, 1))
		if run.rc != 1 {
			t.Errorf("%s: exit = %d, want 1", tc.name, run.rc)
		}
	}
}

func TestClaudePLaunch_RunnerErrorSurfaces(t *testing.T) {
	t.Parallel()
	cfg := &Config{}
	ws := t.TempDir()
	cfg.Workspace, cfg.Worktree, cfg.ProjectRoot, cfg.CLI = ws, ws, ws, "claude-p"
	cfg.PromptFile = filepath.Join(ws, "p.txt")
	_ = os.WriteFile(cfg.PromptFile, []byte("x"), 0o644)
	cfg.StdoutLog, cfg.StderrLog, cfg.Artifact = filepath.Join(ws, "o"), filepath.Join(ws, "e"), filepath.Join(ws, "a")
	deps := Deps{Env: map[string]string{}, Stderr: io.Discard, Runner: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, io.ErrClosedPipe
	}}
	if _, err := (claudePDriver{}).Launch(context.Background(), cfg, deps.withDefaults()); err == nil {
		t.Fatal("Launch error = nil, want the runner error")
	}
}

func TestClaudePLaunch_WritesTheSystemPromptFileAndPassesIt(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prompt := filepath.Join(ws, "pane-authority.md")
	cfg := &Config{Workspace: ws, Realization: RealizeFor("claude-p", LaunchIntent{SettingsScope: "project", SystemPromptFile: prompt, Permission: "bypass", ModelTier: "deep"})}
	run := launchClaudeP(t, cfg, Deps{}, streamRunner("", "", 0))
	if run.rc != 0 {
		t.Fatalf("exit = %d", run.rc)
	}
	if body, err := os.ReadFile(prompt); err != nil || len(body) == 0 {
		t.Fatalf("system prompt file: %v len=%d", err, len(body))
	}
	for _, want := range [][]string{{"--append-system-prompt-file", prompt}, {"--setting-sources", "project"}, {"--model", "opus"}} {
		if !containsRun(run.args, want) {
			t.Errorf("argv %v lacks %v", run.args, want)
		}
	}
	if !containsToken(run.args, "--dangerously-skip-permissions") {
		t.Errorf("argv %v lacks the bypass permission flag", run.args)
	}
}

func TestClaudePLaunch_SystemPromptWriteFailureIsBadFlags(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	blocker := filepath.Join(ws, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o644)
	cfg := &Config{Workspace: ws, Realization: Realization{SystemPromptFile: filepath.Join(blocker, "sub", "pa.md")}}
	cfg.CLI, cfg.Worktree, cfg.ProjectRoot = "claude-p", ws, ws
	cfg.PromptFile = filepath.Join(ws, "p.txt")
	_ = os.WriteFile(cfg.PromptFile, []byte("x"), 0o644)
	cfg.StdoutLog, cfg.StderrLog, cfg.Artifact = filepath.Join(ws, "o"), filepath.Join(ws, "e"), filepath.Join(ws, "a")
	deps := Deps{Env: map[string]string{}, Stderr: io.Discard}
	rc, err := (claudePDriver{}).Launch(context.Background(), cfg, deps.withDefaults())
	if rc != ExitBadFlags || err == nil {
		t.Fatalf("Launch = (%d, %v), want ExitBadFlags with an error", rc, err)
	}
}

func TestClaudePLaunch_ReportsAnOmittedModelToken(t *testing.T) {
	t.Parallel()
	cfg := &Config{Model: "auto", Realization: RealizeFor("claude-p", LaunchIntent{ModelTier: "auto"})}
	run := launchClaudeP(t, cfg, Deps{}, streamRunner("", "", 0))
	if !strings.Contains(run.stderr, "unresolved tier token") || containsToken(run.args, "--model") {
		t.Fatalf("stderr=%q args=%v, want the omission note and no --model", run.stderr, run.args)
	}
}

func TestClaudePLaunch_UnopenableLogsAreBadFlags(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	blocker := filepath.Join(ws, "blocker")
	_ = os.WriteFile(blocker, []byte("x"), 0o644)
	cfg := &Config{CLI: "claude-p", Workspace: ws, Worktree: ws, ProjectRoot: ws, PromptFile: filepath.Join(ws, "p.txt"),
		StdoutLog: filepath.Join(blocker, "o"), StderrLog: filepath.Join(blocker, "e"), Artifact: filepath.Join(ws, "a")}
	_ = os.WriteFile(cfg.PromptFile, []byte("x"), 0o644)
	deps := Deps{Env: map[string]string{}, Stderr: io.Discard}
	if rc, err := (claudePDriver{}).Launch(context.Background(), cfg, deps.withDefaults()); rc != ExitBadFlags || err == nil {
		t.Fatalf("Launch = (%d, %v), want ExitBadFlags with an error", rc, err)
	}
}
