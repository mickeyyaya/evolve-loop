package bridge

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// launch_test.go — M1 parity tests (RED until M2 engine + M3/M5 drivers land).
//
// These port the argv-level behavior of the bash BATS suites onto the
// Go LaunchArgs entry point, using a recording fake runner in place of
// the bash BRIDGE_*_BINARY fake-binary seam:
//
//   tools/agent-bridge/tests/integration/mock-cli-drivers.bats
//   tools/agent-bridge/tests/integration/permission-mode-drivers.bats (non-tmux cases)
//
// The bash tests substitute a fake CLI binary via env and assert on the
// artifact, exit status, and the argv the driver forwarded (captured to
// BRIDGE_FAKE_ARGS_FILE). The Go equivalent injects a fakeRunner via
// Deps.Runner that records every invocation's argv and (optionally)
// simulates the CLI writing the artifact. tmux-driver cases
// (claude-tmux et al.) need the TmuxController seam and land with M4.

// recordedCall captures one CmdRunner invocation for assertions.
type recordedCall struct {
	name string
	args []string
	env  []string
}

// fakeRunner is the test double for the inner-CLI subprocess. It records
// every call and can simulate the CLI writing its artifact. It replaces
// the bash fake-binary seam (fake-claude.sh / fake-codex.sh / fake-agy.sh).
type fakeRunner struct {
	calls []recordedCall
	exit  int
	err   error
	// writeArtifactPath/Body, when set, make the fake write that file on
	// each call — simulating the inner CLI producing its artifact.
	writeArtifactPath string
	writeArtifactBody string
}

func (f *fakeRunner) runner() CmdRunner {
	return func(_ context.Context, name string, args, env []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		f.calls = append(f.calls, recordedCall{name: name, args: append([]string(nil), args...), env: env})
		if f.writeArtifactPath != "" {
			_ = os.MkdirAll(filepath.Dir(f.writeArtifactPath), 0o755)
			_ = os.WriteFile(f.writeArtifactPath, []byte(f.writeArtifactBody), 0o644)
		}
		return f.exit, f.err
	}
}

// argvContainsPair reports whether flag appears in any recorded call's
// argv immediately followed by value — mirroring the BATS check that
// "--permission-mode" is followed by "plan" in the captured args.
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

// --- fixtures -------------------------------------------------------------

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

// launchFixture bundles a workspace + the standard launch arg set, so
// each test mirrors the BATS _run_launch helper.
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

// args builds the bin/bridge-style launch argv for cli, plus any extras.
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

// run drives Engine.LaunchArgs with the fake runner and returns the exit
// code + captured stderr.
func run(t *testing.T, fr *fakeRunner, args []string) (int, string) {
	t.Helper()
	eng := NewEngine(Deps{Runner: fr.runner()})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), args, nil, &stdout, &stderr)
	return code, stderr.String()
}

// --- mock-cli-drivers.bats parity -----------------------------------------

func TestLaunchArgs_ClaudeP_HappyPath(t *testing.T) {
	// T-mock.1: claude-p driver runs the (fake) CLI, artifact is produced
	// with the challenge token, exit 0.
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
	// T-mock.3: codex driver routes to its (fake) CLI and produces the artifact.
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
	// T-mock.4: agy driver routes to its (fake) CLI and produces the artifact.
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

// --- permission-mode-drivers.bats parity (non-tmux) -----------------------

func TestLaunchArgs_ClaudeP_PermissionModePlanReachesInnerArgv(t *testing.T) {
	// T-permmode-drv.1: --permission-mode=plan is forwarded into the
	// claude argv as the pair ["--permission-mode", "plan"].
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
	// T-permmode-drv.15: pass-through works for any valid mode, not just plan.
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
	// T-permmode-drv.5: codex does not support permission_mode → fail with
	// a clear error mentioning permission_mode and (not) supported.
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
	// T-permmode-drv.7: agy does not support permission_mode → clear error.
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
	// T-permmode-drv.9: codex without permission_mode works as before.
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
	// T-permmode-drv.10: agy without permission_mode works as before.
	fx := newFixture(t, "agy", "")
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, _ := run(t, fr, fx.args("agy"))
	if code != ExitOK {
		t.Fatalf("exit = %d, want ExitOK", code)
	}
}

// --- launch validation (bin/bridge cmd_launch required-field guards) ------

func TestLaunchArgs_MissingRequiredFlag(t *testing.T) {
	// bin/bridge: missing a required flag (here --cli) → EC_BAD_FLAGS with
	// a message naming the missing field.
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
	// bin/bridge: no driver for cli=X → EC_BAD_FLAGS ("no driver for cli=").
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
	// bin/bridge F5: an empty prompt file fails fast (would otherwise hang
	// the agent at the artifact timeout) → EC_BAD_FLAGS.
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
