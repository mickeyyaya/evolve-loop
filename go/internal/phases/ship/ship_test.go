// Tests for the ship phase. These tests cover the LEGACY shell-out path
// (EVOLVE_NATIVE_SHIP=0). The v11.3.0 native Go path is exercised by
// native_test.go. The legacy path tests inject a fake CmdRunner to
// avoid spawning subprocesses.
package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fakeCmd captures the most recent invocation and produces scripted output.
type fakeCmd struct {
	exitCode int
	err      error
	stdout   string
	stderr   string

	gotName string
	gotArgs []string
	gotEnv  []string
	gotCWD  string
	calls   int
}

func (f *fakeCmd) runner() CmdRunner {
	return func(ctx context.Context, name string, args, env []string, cwd string,
		stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		f.calls++
		f.gotName = name
		f.gotArgs = append([]string{}, args...)
		f.gotEnv = append([]string{}, env...)
		f.gotCWD = cwd
		if f.stdout != "" {
			_, _ = stdout.Write([]byte(f.stdout))
		}
		if f.stderr != "" {
			_, _ = stderr.Write([]byte(f.stderr))
		}
		return f.exitCode, f.err
	}
}

func fixedClock(t time.Time, dur time.Duration) func() time.Time {
	calls := 0
	return func() time.Time {
		defer func() { calls++ }()
		if calls == 0 {
			return t
		}
		return t.Add(dur)
	}
}

func TestRun_HappyPath_PASS(t *testing.T) {
	ws := t.TempDir()
	fc := &fakeCmd{
		exitCode: 0,
		stdout: `[ship] OK: committed to feat-branch
[ship] OK: pushed to origin/feat-branch
[ship] DONE: shipped feat-branch at a1b2c3d4e5f60718293a4b5c6d7e8f90123456ab
`,
	}
	clock := fixedClock(time.Unix(1_700_000_000, 0), 250*time.Millisecond)
	phase := New(Config{
		Runner: fc.runner(),
		NowFn:  clock,
	})

	req := core.PhaseRequest{
		Cycle:       42,
		ProjectRoot: "/tmp/proj",
		Workspace:   ws,
		Context:     map[string]string{"commit_message": "feat: rate limit /login"},
		Env:         map[string]string{"EVOLVE_NATIVE_SHIP": "0"},
	}
	resp, err := phase.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict=%q, want PASS", resp.Verdict)
	}
	if resp.NextPhase != "retro" {
		t.Errorf("NextPhase=%q, want retro", resp.NextPhase)
	}
	if resp.DurationMS != 250 {
		t.Errorf("DurationMS=%d, want 250", resp.DurationMS)
	}
	// Inspect captured args.
	if !strings.HasSuffix(fc.gotName, "legacy/scripts/lifecycle/ship.sh") &&
		!strings.HasSuffix(fc.gotName, "ship.sh") {
		t.Errorf("invoked %q, want legacy/scripts/lifecycle/ship.sh", fc.gotName)
	}
	wantArgs := []string{"--class", "cycle", "feat: rate limit /login"}
	if len(fc.gotArgs) != len(wantArgs) {
		t.Errorf("argv=%v, want %v", fc.gotArgs, wantArgs)
	} else {
		for i := range wantArgs {
			if fc.gotArgs[i] != wantArgs[i] {
				t.Errorf("arg[%d]=%q, want %q", i, fc.gotArgs[i], wantArgs[i])
			}
		}
	}
	if fc.gotCWD != "/tmp/proj" {
		t.Errorf("CWD=%q, want /tmp/proj", fc.gotCWD)
	}
}

func TestRun_ShipScriptFails_FAIL(t *testing.T) {
	fc := &fakeCmd{
		exitCode: 2,
		stderr:   "[ship-gate] DENIED: ledger chain broken\n",
	}
	phase := New(Config{Runner: fc.runner()})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"commit_message": "x"},
		Env:     map[string]string{"EVOLVE_NATIVE_SHIP": "0"},
	})
	if err == nil {
		t.Fatal("err=nil, want non-nil for exit=2")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if len(resp.Diagnostics) == 0 || !strings.Contains(resp.Diagnostics[0].Message, "ledger chain broken") {
		t.Errorf("expected stderr in Diagnostics; got %+v", resp.Diagnostics)
	}
}

func TestRun_RunnerError_FAIL(t *testing.T) {
	runErr := errors.New("exec failed")
	fc := &fakeCmd{err: runErr}
	phase := New(Config{Runner: fc.runner()})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
		Context: map[string]string{"commit_message": "x"},
		Env:     map[string]string{"EVOLVE_NATIVE_SHIP": "0"},
	})
	if !errors.Is(err, runErr) {
		t.Errorf("err=%v, want runErr", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
}

func TestRun_MissingCommitMessage_FAIL(t *testing.T) {
	fc := &fakeCmd{exitCode: 0}
	phase := New(Config{Runner: fc.runner()})
	resp, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p", Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("err=nil, want non-nil for missing commit_message")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("Verdict=%q, want FAIL", resp.Verdict)
	}
	if fc.calls != 0 {
		t.Errorf("CmdRunner called %d times; want 0 (missing commit message must short-circuit)", fc.calls)
	}
}

func TestRun_MissingRunner_ReturnsError(t *testing.T) {
	phase := New(Config{})
	_, err := phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p",
		Context: map[string]string{"commit_message": "x"},
	})
	if err == nil || !strings.Contains(err.Error(), "runner required") {
		t.Fatalf("err=%v, want runner-required", err)
	}
}

func TestRun_AlternateShipScript_HonorsEnvOverride(t *testing.T) {
	fc := &fakeCmd{exitCode: 0, stdout: "[ship] DONE\n"}
	phase := New(Config{Runner: fc.runner()})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p",
		Workspace: t.TempDir(),
		Context:   map[string]string{"commit_message": "x"},
		Env: map[string]string{
			"EVOLVE_NATIVE_SHIP": "0",
			"EVOLVE_SHIP_SCRIPT": "/custom/ship.sh",
		},
	})
	if fc.gotName != "/custom/ship.sh" {
		t.Errorf("name=%q, want /custom/ship.sh", fc.gotName)
	}
}

func TestRun_EnvBypassPropagated(t *testing.T) {
	fc := &fakeCmd{exitCode: 0, stdout: "ok"}
	phase := New(Config{Runner: fc.runner()})
	_, _ = phase.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, ProjectRoot: "/p",
		Workspace: t.TempDir(),
		Context:   map[string]string{"commit_message": "x"},
		Env: map[string]string{
			"EVOLVE_NATIVE_SHIP":        "0",
			"EVOLVE_SHIP_AUTO_CONFIRM":  "1",
			"EVOLVE_BYPASS_PREFIX_GATE": "1",
		},
	})
	foundAuto := false
	foundBypass := false
	for _, kv := range fc.gotEnv {
		if kv == "EVOLVE_SHIP_AUTO_CONFIRM=1" {
			foundAuto = true
		}
		if kv == "EVOLVE_BYPASS_PREFIX_GATE=1" {
			foundBypass = true
		}
	}
	if !foundAuto || !foundBypass {
		t.Errorf("env not propagated; got=%v", fc.gotEnv)
	}
}

func TestRun_ShipReportArtifactSurfaces(t *testing.T) {
	ws := t.TempDir()
	// Pre-create a ship-report.md to mimic ship.sh writing it.
	_ = os.WriteFile(filepath.Join(ws, "ship-report.md"), []byte("commit: deadbeef\n"), 0o644)
	fc := &fakeCmd{exitCode: 0, stdout: "[ship] DONE"}
	phase := New(Config{Runner: fc.runner()})
	resp, _ := phase.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: "/p",
		Workspace:   ws,
		Context:     map[string]string{"commit_message": "x"},
		Env:         map[string]string{"EVOLVE_NATIVE_SHIP": "0"},
	})
	if resp.ArtifactsDir != ws {
		t.Errorf("ArtifactsDir=%q, want %q", resp.ArtifactsDir, ws)
	}
}

func TestName(t *testing.T) {
	p := New(Config{})
	if p.Name() != "ship" {
		t.Errorf("Name=%q, want ship", p.Name())
	}
}

// TestExecRunner_Success drives the production CmdRunner against
// /bin/true (exit 0). Locked to POSIX paths; tests skip on Windows
// (we don't ship Windows per parent plan §7).
func TestExecRunner_Success(t *testing.T) {
	if _, err := os.Stat("/bin/true"); err != nil {
		t.Skip("no /bin/true")
	}
	var stdout, stderr io.Writer = io.Discard, io.Discard
	code, err := execRunner(context.Background(), "/bin/true", nil, nil, "", nil, stdout, stderr)
	if err != nil {
		t.Fatalf("execRunner: %v", err)
	}
	if code != 0 {
		t.Errorf("exit=%d, want 0", code)
	}
}

func TestExecRunner_NonZeroExit(t *testing.T) {
	if _, err := os.Stat("/bin/false"); err != nil {
		t.Skip("no /bin/false")
	}
	code, err := execRunner(context.Background(), "/bin/false", nil, nil, "", nil, io.Discard, io.Discard)
	if err != nil {
		t.Errorf("err=%v, want nil (exit-status mapped to code)", err)
	}
	if code == 0 {
		t.Errorf("exit=%d, want non-zero", code)
	}
}

func TestExecRunner_NotFound(t *testing.T) {
	_, err := execRunner(context.Background(), "/no/such/binary/ever", nil, nil, "", nil, io.Discard, io.Discard)
	if err == nil {
		t.Errorf("err=nil, want non-nil for missing binary")
	}
}

func TestNewWithDefaultRunner_HasRunner(t *testing.T) {
	p := NewWithDefaultRunner()
	if p == nil {
		t.Fatal("NewWithDefaultRunner returned nil")
	}
	if p.runner == nil {
		t.Errorf("runner field is nil; want execRunner")
	}
}
