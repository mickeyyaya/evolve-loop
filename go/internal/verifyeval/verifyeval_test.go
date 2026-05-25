package verifyeval

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEval(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "eval.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeRunner returns scripted outputs keyed by command. Tests inject
// it via Options.Runner to drive Verify without real shell execution.
type fakeRunner struct {
	scripts map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}
	calls []string
}

func (f *fakeRunner) run() CmdRunner {
	return func(_ context.Context, _, cmd string) (string, string, int, error) {
		f.calls = append(f.calls, cmd)
		s, ok := f.scripts[cmd]
		if !ok {
			return "", "", 0, nil
		}
		return s.stdout, s.stderr, s.exit, s.err
	}
}

// TestVerify_HappyPath_PASS — all commands pass expectations.
func TestVerify_HappyPath_PASS(t *testing.T) {
	path := writeEval(t, "```bash\ngo test ./...\n```\n\n## Expected\n\nexit_code: 0\nstdout_contains: ok\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"go test ./...": {stdout: "ok  example.com/pkg 0.5s\n", exit: 0},
	}}
	res, err := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "PASS" {
		t.Errorf("Verdict=%q, want PASS; commands=%+v", res.Verdict, res.Commands)
	}
}

// TestVerify_ExitCodeMismatch_FAIL — non-matching exit code flips
// verdict to FAIL with a descriptive reason.
func TestVerify_ExitCodeMismatch_FAIL(t *testing.T) {
	path := writeEval(t, "```bash\nrun\n```\n\n## Expected\n\nexit_code: 0\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"run": {exit: 1},
	}}
	res, _ := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if res.Verdict != "FAIL" {
		t.Errorf("Verdict=%q, want FAIL", res.Verdict)
	}
	if !strings.Contains(res.Commands[0].Reason, "exit_code=1") {
		t.Errorf("Reason missing exit_code detail: %q", res.Commands[0].Reason)
	}
}

// TestVerify_StdoutContainsMismatch_FAIL — expected substring absent.
func TestVerify_StdoutContainsMismatch_FAIL(t *testing.T) {
	path := writeEval(t, "```bash\nrun\n```\n\n## Expected\n\nstdout_contains: hello\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"run": {stdout: "world"},
	}}
	res, _ := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if res.Verdict != "FAIL" {
		t.Errorf("Verdict=%q, want FAIL", res.Verdict)
	}
}

// TestVerify_StdoutAbsent_Mismatch — string present when it shouldn't be.
func TestVerify_StdoutAbsent_Mismatch(t *testing.T) {
	path := writeEval(t, "```bash\nrun\n```\n\n## Expected\n\nstdout_absent: FAIL\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"run": {stdout: "tests FAILed"},
	}}
	res, _ := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if res.Verdict != "FAIL" {
		t.Errorf("Verdict=%q, want FAIL", res.Verdict)
	}
}

// TestVerify_StderrContains — stderr predicate works the same as stdout.
func TestVerify_StderrContains(t *testing.T) {
	path := writeEval(t, "```bash\nrun\n```\n\n## Expected\n\nstderr_contains: panic\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"run": {stderr: "got a panic in goroutine 1"},
	}}
	res, _ := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if res.Verdict != "PASS" {
		t.Errorf("Verdict=%q, want PASS (stderr contained panic)", res.Verdict)
	}
}

// TestVerify_RunnerError_FAIL — runner returning err marks the command
// failed but continues to subsequent commands.
func TestVerify_RunnerError_FAIL(t *testing.T) {
	path := writeEval(t, "```bash\nbad\ngood\n```\n\n## Expected\n\nexit_code: 0\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"bad":  {err: errors.New("spawn failed")},
		"good": {exit: 0},
	}}
	res, _ := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if res.Verdict != "FAIL" {
		t.Errorf("Verdict=%q, want FAIL", res.Verdict)
	}
	if len(res.Commands) != 2 {
		t.Errorf("expected 2 command results (continued past failure); got %d", len(res.Commands))
	}
}

// TestVerify_MissingPath_Error — required-field validation.
func TestVerify_MissingPath_Error(t *testing.T) {
	if _, err := Verify(Options{Workspace: "/tmp"}); err == nil {
		t.Error("Verify with empty Path: want error")
	}
}

// TestVerify_MissingWorkspace_Error — required-field validation.
func TestVerify_MissingWorkspace_Error(t *testing.T) {
	if _, err := Verify(Options{Path: "/tmp/x"}); err == nil {
		t.Error("Verify with empty Workspace: want error")
	}
}

// TestVerify_FileNotFound_Error — open failure surfaces as error.
func TestVerify_FileNotFound_Error(t *testing.T) {
	if _, err := Verify(Options{Path: "/no/such/file.md", Workspace: "/tmp"}); err == nil {
		t.Error("Verify on missing file: want error")
	}
}

// TestVerify_NoCommands_PASS — eval with no bash blocks PASSes
// vacuously (nothing to check).
func TestVerify_NoCommands_PASS(t *testing.T) {
	path := writeEval(t, "## Expected\n\nexit_code: 0\n")
	res, err := Verify(Options{Path: path, Workspace: "/tmp", Runner: (&fakeRunner{}).run()})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verdict != "PASS" {
		t.Errorf("Verdict=%q, want PASS (vacuous)", res.Verdict)
	}
}

// TestParseExpectedLine_AllKeys — direct unit test of the parser
// because the format is the operator-facing contract.
func TestParseExpectedLine_AllKeys(t *testing.T) {
	var e Expectations
	parseExpectedLine("exit_code: 42", &e)
	parseExpectedLine(`stdout_contains: "needle"`, &e)
	parseExpectedLine("stdout_absent: 'noise'", &e)
	parseExpectedLine("stderr_contains: err", &e)
	parseExpectedLine("unknown_key: ignored", &e)
	parseExpectedLine("no_colon_line", &e)

	if e.ExitCode == nil || *e.ExitCode != 42 {
		t.Errorf("ExitCode = %v, want 42", e.ExitCode)
	}
	if e.StdoutContains != "needle" {
		t.Errorf("StdoutContains=%q, want needle", e.StdoutContains)
	}
	if e.StdoutAbsent != "noise" {
		t.Errorf("StdoutAbsent=%q, want noise", e.StdoutAbsent)
	}
	if e.StderrContains != "err" {
		t.Errorf("StderrContains=%q, want err", e.StderrContains)
	}
}

// TestParseExpectedLine_MalformedExitCode — unparseable int is silently
// ignored so a malformed eval doesn't crash the verifier.
func TestParseExpectedLine_MalformedExitCode(t *testing.T) {
	var e Expectations
	parseExpectedLine("exit_code: not-a-number", &e)
	if e.ExitCode != nil {
		t.Errorf("ExitCode = %v, want nil (malformed)", e.ExitCode)
	}
}

// TestMatchExpectations_AllPathsDirect — direct table coverage of
// matchExpectations covering each predicate's pass + fail branches.
func TestMatchExpectations_AllPathsDirect(t *testing.T) {
	mkInt := func(n int) *int { return &n }
	cases := []struct {
		name      string
		cr        CommandResult
		e         Expectations
		wantEmpty bool // true → predicate passed (empty reason)
	}{
		{"exit-pass", CommandResult{ExitCode: 0}, Expectations{ExitCode: mkInt(0)}, true},
		{"exit-fail", CommandResult{ExitCode: 1}, Expectations{ExitCode: mkInt(0)}, false},
		{"stdout-contains-pass", CommandResult{Stdout: "hello"}, Expectations{StdoutContains: "ell"}, true},
		{"stdout-contains-fail", CommandResult{Stdout: "hi"}, Expectations{StdoutContains: "ell"}, false},
		{"stdout-absent-pass", CommandResult{Stdout: "ok"}, Expectations{StdoutAbsent: "FAIL"}, true},
		{"stdout-absent-fail", CommandResult{Stdout: "FAIL!"}, Expectations{StdoutAbsent: "FAIL"}, false},
		{"stderr-contains-pass", CommandResult{Stderr: "warn"}, Expectations{StderrContains: "warn"}, true},
		{"stderr-contains-fail", CommandResult{Stderr: "ok"}, Expectations{StderrContains: "warn"}, false},
		{"all-empty-empty", CommandResult{}, Expectations{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchExpectations(c.cr, c.e)
			if c.wantEmpty && got != "" {
				t.Errorf("want empty reason; got %q", got)
			}
			if !c.wantEmpty && got == "" {
				t.Errorf("want non-empty reason; got empty")
			}
		})
	}
}

// TestParseEval_BashBlockAfterExpected — a bash block reopened after
// the Expected section still emits its commands (regression: prior
// behavior incorrectly stayed in inExpected mode).
func TestParseEval_BashBlockAfterExpected(t *testing.T) {
	path := writeEval(t, "## Expected\nexit_code: 0\n\n## Setup\n```bash\nrun\n```\n")
	cmds, _, err := parseEval(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cmds {
		if c == "run" {
			found = true
		}
	}
	if !found {
		t.Errorf("commands missing 'run'; got %v", cmds)
	}
}

// TestVerify_MultipleBashBlocks_AllRun — every block contributes
// commands; expectations apply to each.
func TestVerify_MultipleBashBlocks_AllRun(t *testing.T) {
	path := writeEval(t, "```bash\nfirst\n```\n## Section\n```bash\nsecond\n```\n\n## Expected\n\nexit_code: 0\n")
	fr := &fakeRunner{scripts: map[string]struct {
		stdout, stderr string
		exit           int
		err            error
	}{
		"first":  {exit: 0},
		"second": {exit: 0},
	}}
	res, _ := Verify(Options{Path: path, Workspace: "/tmp", Runner: fr.run()})
	if len(res.Commands) != 2 {
		t.Errorf("len=%d, want 2", len(res.Commands))
	}
	if res.Verdict != "PASS" {
		t.Errorf("Verdict=%q, want PASS", res.Verdict)
	}
}
