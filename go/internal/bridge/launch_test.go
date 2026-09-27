package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordedCall captures one CmdRunner invocation for assertions.
type recordedCall struct {
	name string
	dir  string
	args []string
	env  []string
}

// fakeRunner is the test double for the inner-CLI subprocess: it records every call and can simulate the
// CLI writing its artifact.
type fakeRunner struct {
	calls []recordedCall
	exit  int
	err   error
	// writeArtifactPath/Body, when set, make the fake write that file on each call.
	writeArtifactPath string
	writeArtifactBody string
}

func (f *fakeRunner) runner() CmdRunner {
	return func(_ context.Context, name, dir string, args, env []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		f.calls = append(f.calls, recordedCall{name: name, dir: dir, args: append([]string(nil), args...), env: env})
		if f.writeArtifactPath != "" {
			_ = os.MkdirAll(filepath.Dir(f.writeArtifactPath), 0o755)
			_ = os.WriteFile(f.writeArtifactPath, []byte(f.writeArtifactBody), 0o644)
		}
		return f.exit, f.err
	}
}

// argvContainsPair reports whether flag appears in any recorded call's argv immediately followed by value.
func (f *fakeRunner) argvContainsPair(flag, value string) bool {
	for _, c := range f.calls {
		for i := 0; i < len(c.args)-1; i++ {
			if c.args[i] == flag && c.args[i+1] == value {
				return true
			}
		}
	}
	return false
}

// writeProfile writes a minimal valid profile JSON and returns its path.
// permissionMode "" omits the field (back-compat profile).
func writeProfile(t *testing.T, dir, name, permissionMode string) string {
	t.Helper()
	var perm string
	if permissionMode != "" {
		perm = `"permission_mode": "` + permissionMode + `",`
	}
	body := `{
  "name": "` + name + `",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  ` + perm + `
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": []
}
`
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}
	return path
}

// launchFixture bundles a workspace and the standard launch arg set.
type launchFixture struct {
	ws         string
	profile    string
	promptFile string
	artifact   string
	stdoutLog  string
	stderrLog  string
	token      string
}

func newFixture(t *testing.T, cli, permissionMode string) launchFixture {
	t.Helper()
	ws := t.TempDir()
	token := "tok-" + cli
	promptFile := filepath.Join(ws, "prompt.txt")
	body := "Use your Write tool to create artifact containing:\n<!-- challenge-token: " + token + " -->\nPROTOTYPE OK\n"
	if err := os.WriteFile(promptFile, []byte(body), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	return launchFixture{
		ws:         ws,
		profile:    writeProfile(t, ws, "test-"+cli, permissionMode),
		promptFile: promptFile,
		artifact:   filepath.Join(ws, "artifact.md"),
		stdoutLog:  filepath.Join(ws, "stdout.log"),
		stderrLog:  filepath.Join(ws, "stderr.log"),
		token:      token,
	}
}

// args builds the launch argv for cli, plus any extras.
func (fx launchFixture) args(cli string, extra ...string) []string {
	base := []string{
		"--cli=" + cli,
		"--profile=" + fx.profile,
		"--model=auto",
		"--prompt-file=" + fx.promptFile,
		"--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog,
		"--stderr-log=" + fx.stderrLog,
		"--artifact=" + fx.artifact,
	}
	return append(base, extra...)
}

// run drives Engine.LaunchArgs with the fake runner and an empty env lookup, so the credential-isolation
// guards never fire from ambient process env, keeping these tests deterministic on any machine. Tests that
// exercise the guards use runLookup (driver_credentials_test.go).
func run(t *testing.T, fr *fakeRunner, args []string) (int, string) {
	t.Helper()
	return runLookup(t, fr, args, nil)
}

// newTestEngine builds an Engine for driver tests with a hermetic environment by default: a test that does
// not pin Deps.LookupEnv gets the empty lookup rather than falling through to os.LookupEnv, carrying the
// `run`/`runLookup` convention above across to the tmux-REPL suite. Deps.Env is consulted first by
// lookupEnv and is unaffected, so a test that means to exercise an env branch still opts in explicitly.
func newTestEngine(d Deps) *Engine {
	if d.LookupEnv == nil {
		d.LookupEnv = mapLookup(nil)
	}
	return NewEngine(d)
}

func TestLaunchArgs_ClaudeP_HappyPath(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "<!-- challenge-token: " + fx.token + " -->\nOK\n"}
	code, _ := run(t, fr, fx.args("claude-p"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d (ExitOK)", code, ExitOK)
	}
	if len(fr.calls) == 0 {
		t.Fatalf("driver did not invoke the inner CLI runner")
	}
	got, _ := os.ReadFile(fx.artifact)
	if !strings.Contains(string(got), fx.token) {
		t.Fatalf("artifact missing challenge token %q; got %q", fx.token, string(got))
	}
}

func TestLaunchArgs_Codex_HappyPath(t *testing.T) {
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "<!-- challenge-token: " + fx.token + " -->\nFAKE-CODEX\n"}
	code, _ := run(t, fr, fx.args("codex"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if len(fr.calls) == 0 {
		t.Fatalf("codex driver did not invoke the inner CLI runner")
	}
	if _, err := os.Stat(fx.artifact); err != nil {
		t.Fatalf("artifact not produced: %v", err)
	}
}

func TestLaunchArgs_Agy_HappyPath(t *testing.T) {
	fx := newFixture(t, "agy", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "<!-- challenge-token: " + fx.token + " -->\nFAKE-AGY\n"}
	code, _ := run(t, fr, fx.args("agy"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if len(fr.calls) == 0 {
		t.Fatalf("agy driver did not invoke the inner CLI runner")
	}
}

func TestLaunchArgs_ClaudeP_PermissionModePlanReachesInnerArgv(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := run(t, fr, fx.args("claude-p", "--permission-mode=plan"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if !fr.argvContainsPair("--permission-mode", "plan") {
		t.Fatalf("inner argv missing [--permission-mode plan]; calls=%+v", fr.calls)
	}
}

func TestLaunchArgs_ClaudeP_PermissionModeAcceptEditsReachesInnerArgv(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := run(t, fr, fx.args("claude-p", "--permission-mode=acceptEdits"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if !fr.argvContainsPair("--permission-mode", "acceptEdits") {
		t.Fatalf("inner argv missing [--permission-mode acceptEdits]; calls=%+v", fr.calls)
	}
}

func TestLaunchArgs_Codex_PermissionModeRejected(t *testing.T) {
	fx := newFixture(t, "codex", "plan")
	fr := &fakeRunner{}
	code, stderr := run(t, fr, fx.args("codex"))
	if code == ExitOK {
		t.Fatalf("exit = ExitOK, want non-zero rejection")
	}
	if !strings.Contains(stderr, "permission_mode") {
		t.Fatalf("stderr should mention permission_mode; got %q", stderr)
	}
	if !strings.Contains(stderr, "not supported") && !strings.Contains(stderr, "unsupported") {
		t.Fatalf("stderr should say (not) supported; got %q", stderr)
	}
}

func TestLaunchArgs_Agy_PermissionModeRejected(t *testing.T) {
	fx := newFixture(t, "agy", "plan")
	fr := &fakeRunner{}
	code, stderr := run(t, fr, fx.args("agy"))
	if code == ExitOK {
		t.Fatalf("exit = ExitOK, want non-zero rejection")
	}
	if !strings.Contains(stderr, "permission_mode") {
		t.Fatalf("stderr should mention permission_mode; got %q", stderr)
	}
	if !strings.Contains(stderr, "not supported") && !strings.Contains(stderr, "unsupported") {
		t.Fatalf("stderr should say (not) supported; got %q", stderr)
	}
}

func TestLaunchArgs_Codex_NoPermissionModeBackCompat(t *testing.T) {
	fx := newFixture(t, "codex", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := run(t, fr, fx.args("codex"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
	if _, err := os.Stat(fx.artifact); err != nil {
		t.Fatalf("artifact not produced: %v", err)
	}
}

func TestLaunchArgs_Agy_NoPermissionModeBackCompat(t *testing.T) {
	fx := newFixture(t, "agy", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := run(t, fr, fx.args("agy"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
}

// noSandboxWrap returns a SandboxWrapper that always declines, so the headless drivers run the inner CLI
// unwrapped, keeping the recorded (name, dir, args) clean for the assertion; cfg.Worktree alone would
// otherwise invite the real sandbox probe on hosts that can wrap.
func noSandboxWrap() SandboxWrapper {
	return func(SandboxWrapRequest) ([]string, bool) { return nil, false }
}

// runWithSandbox drives LaunchArgs with the fake runner, an empty env lookup, and an injected SandboxWrap
// so the run is deterministic on any host; mirrors run() but threads the sandbox seam.
func runWithSandbox(t *testing.T, fr *fakeRunner, sw SandboxWrapper, args []string) (int, string) {
	t.Helper()
	var stderr strings.Builder
	eng := NewEngine(Deps{Runner: fr.runner(), LookupEnv: mapLookup(nil), SandboxWrap: sw})
	code := eng.LaunchArgs(context.Background(), args, nil, io.Discard, &stderr)
	return code, stderr.String()
}

func TestLaunchArgs_HeadlessDrivers_RunInWorktree(t *testing.T) {
	// The headless drivers must set the subprocess cwd to cfg.Worktree for source-writing phases, parity
	// with the tmux driver's `cd <worktree>`: without this, a build agent writes files to the main tree
	// and the tree-diff guard aborts the cycle.
	for _, cli := range []string{"claude-p", "codex", "agy"} {
		t.Run(cli+"/worktree-set", func(t *testing.T) {
			fx := newFixture(t, cli, "")
			worktree := t.TempDir()
			fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
			code, stderr := runWithSandbox(t, fr, noSandboxWrap(), fx.args(cli, "--worktree="+worktree))
			if code != ExitOK {
				t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
			}
			if len(fr.calls) == 0 {
				t.Fatalf("driver did not invoke the inner CLI runner")
			}
			if got := fr.calls[0].dir; got != worktree {
				t.Fatalf("runner dir = %q, want worktree %q", got, worktree)
			}
		})
		t.Run(cli+"/worktree-empty", func(t *testing.T) {
			// No --worktree → cfg.Worktree=="" → dir=="": inherits the caller cwd for non-source-writing phases.
			fx := newFixture(t, cli, "")
			fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
			code, stderr := runWithSandbox(t, fr, noSandboxWrap(), fx.args(cli))
			if code != ExitOK {
				t.Fatalf("exit = %d, want ExitOK; stderr=%q", code, stderr)
			}
			if len(fr.calls) == 0 {
				t.Fatalf("driver did not invoke the inner CLI runner")
			}
			if got := fr.calls[0].dir; got != "" {
				t.Fatalf("runner dir = %q, want empty (inherit caller cwd)", got)
			}
		})
	}
}

func TestLaunchArgs_MissingRequiredFlag(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	fr := &fakeRunner{}
	// Build args WITHOUT --cli.
	args := []string{
		"--profile=" + fx.profile,
		"--model=auto",
		"--prompt-file=" + fx.promptFile,
		"--workspace=" + fx.ws,
		"--stdout-log=" + fx.stdoutLog,
		"--stderr-log=" + fx.stderrLog,
		"--artifact=" + fx.artifact,
	}
	code, stderr := run(t, fr, args)
	if code != ExitBadFlags {
		t.Fatalf("exit = %d, want %d (ExitBadFlags)", code, ExitBadFlags)
	}
	if !strings.Contains(stderr, "cli") {
		t.Fatalf("stderr should name the missing --cli field; got %q", stderr)
	}
}

func TestLaunchArgs_UnknownCLI(t *testing.T) {
	fx := newFixture(t, "nope", "")
	fr := &fakeRunner{}
	code, stderr := run(t, fr, fx.args("nope"))
	if code != ExitBadFlags {
		t.Fatalf("exit = %d, want ExitBadFlags", code)
	}
	if !strings.Contains(stderr, "nope") {
		t.Fatalf("stderr should name the unknown cli; got %q", stderr)
	}
}

func TestLaunchArgs_EmptyPromptFile(t *testing.T) {
	fx := newFixture(t, "claude-p", "")
	if err := os.WriteFile(fx.promptFile, nil, 0o644); err != nil {
		t.Fatalf("truncate prompt: %v", err)
	}
	fr := &fakeRunner{}
	code, stderr := run(t, fr, fx.args("claude-p"))
	if code != ExitBadFlags {
		t.Fatalf("exit = %d, want ExitBadFlags", code)
	}
	if !strings.Contains(stderr, "empty") {
		t.Fatalf("stderr should mention the empty prompt; got %q", stderr)
	}
}
