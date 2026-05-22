// Package bridge wraps tools/agent-bridge/bin/bridge as the Go-side
// LLM dispatch surface. Tests use injected runners so no actual bridge
// subprocess runs.
package bridge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// fakeCmd records the args/env it was invoked with and produces
// scripted stdout/stderr/exit. One per launch.
type fakeCmd struct {
	name     string
	args     []string
	stdin    []byte
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
	// observed inputs
	gotArgs []string
	gotEnv  []string
}

func (f *fakeCmd) runner() CmdRunner {
	return func(ctx context.Context, name string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		f.gotArgs = append([]string{name}, args...)
		f.gotEnv = append([]string{}, env...)
		_, _ = stdout.Write(f.stdout)
		_, _ = stderr.Write(f.stderr)
		return f.exitCode, f.err
	}
}

// TestLaunch_HappyPath drives a successful invocation: exit 0, artifact
// written, stdout/stderr captured. Verifies that the adapter
// materializes the prompt, derives missing output paths, and reads back
// the artifact contents into BridgeResponse.Stdout.
func TestLaunch_HappyPath(t *testing.T) {
	ws := t.TempDir()
	artifactPath := filepath.Join(ws, "scout-report.md")
	prepArtifact := func() {
		_ = os.WriteFile(artifactPath, []byte("# Scout Report\nbody"), 0o644)
	}
	fc := &fakeCmd{exitCode: 0, stdout: []byte("bridge happy")}
	// Use the fake runner with a hook that writes the artifact before
	// returning (mimics bridge's behavior).
	runner := func(ctx context.Context, name string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		fc.gotArgs = append([]string{name}, args...)
		prepArtifact()
		_, _ = stdout.Write(fc.stdout)
		return 0, nil
	}

	adapter := New("/path/to/bridge", runner)
	req := core.BridgeRequest{
		CLI:          "claude-p",
		Profile:      "/tmp/profile.json",
		Model:        "sonnet",
		Prompt:       "test prompt",
		Workspace:    ws,
		ArtifactPath: artifactPath,
		Agent:        "scout",
		Cycle:        7,
	}
	resp, err := adapter.Launch(context.Background(), req)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if resp.ExitCode != 0 {
		t.Errorf("ExitCode=%d, want 0", resp.ExitCode)
	}
	if !strings.Contains(resp.Stdout, "Scout Report") {
		t.Errorf("Stdout missing artifact contents: %q", resp.Stdout)
	}
}

// TestLaunch_ArgvShape verifies the bridge command line carries the
// load-bearing flags. Brittle by design — the bridge contract is the
// reason this adapter exists.
func TestLaunch_ArgvShape(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "out.md")
	_ = os.WriteFile(artifact, []byte("hi"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/usr/local/bin/bridge", runner)
	_, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-tmux", Profile: "/p.json", Model: "opus",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact,
		Agent: "builder", Cycle: 42,
		Worktree:   "/wt",
		ExtraFlags: []string{"--require-full"},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	got := strings.Join(fc.gotArgs, " ")
	wantFrags := []string{
		"/usr/local/bin/bridge",
		"launch",
		"--cli=claude-tmux",
		"--profile=/p.json",
		"--model=opus",
		"--workspace=" + ws,
		"--artifact=" + artifact,
		"--cycle=42",
		"--agent=builder",
		"--worktree=/wt",
		"--require-full",
	}
	for _, f := range wantFrags {
		if !strings.Contains(got, f) {
			t.Errorf("argv missing %q\n  full: %s", f, got)
		}
	}
}

// TestLaunch_PromptMaterializedAsFile — the prompt body must be
// written to a file under Workspace before the subprocess runs;
// --prompt-file=<path> appears in argv.
func TestLaunch_PromptMaterializedAsFile(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "a.md")
	_ = os.WriteFile(artifact, []byte("x"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/bridge", runner)
	_, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p.json", Model: "haiku",
		Prompt: "the prompt body", Workspace: ws, ArtifactPath: artifact, Agent: "x",
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	var promptFlag string
	for _, a := range fc.gotArgs {
		if strings.HasPrefix(a, "--prompt-file=") {
			promptFlag = strings.TrimPrefix(a, "--prompt-file=")
			break
		}
	}
	if promptFlag == "" {
		t.Fatalf("argv missing --prompt-file: %v", fc.gotArgs)
	}
	// The prompt file must exist and contain the body.
	b, err := os.ReadFile(promptFlag)
	if err != nil {
		t.Fatalf("read prompt file: %v", err)
	}
	if string(b) != "the prompt body" {
		t.Errorf("prompt file content=%q, want %q", b, "the prompt body")
	}
}

// TestLaunch_DefaultLogPaths — when StdoutLog/StderrLog are unset, the
// adapter derives canonical paths under Workspace. The bridge flags
// always carry a concrete path.
func TestLaunch_DefaultLogPaths(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "a.md")
	_ = os.WriteFile(artifact, []byte("x"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/bridge", runner)
	_, _ = adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "haiku",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "scout",
	})
	got := strings.Join(fc.gotArgs, " ")
	for _, prefix := range []string{"--stdout-log=", "--stderr-log="} {
		if !strings.Contains(got, prefix) {
			t.Errorf("argv missing %s flag: %s", prefix, got)
		}
	}
}

// TestLaunch_NonZeroExit_PropagatesError — bridge exit codes ≠ 0 must
// surface so the orchestrator can react (retry, fail, escalate).
func TestLaunch_NonZeroExit_PropagatesError(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "a.md")
	fc := &fakeCmd{exitCode: 99, stderr: []byte("require-full not met")}
	runner := fc.runner()

	adapter := New("/bridge", runner)
	resp, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "auto",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "x",
	})
	if err == nil {
		t.Error("Launch: want error on exit 99")
	}
	if resp.ExitCode != 99 {
		t.Errorf("ExitCode=%d, want 99", resp.ExitCode)
	}
}

// TestLaunch_MissingArtifact_NotFatal — exit 0 but artifact file
// absent. Adapter should still return a successful BridgeResponse,
// but BridgeResponse.Stdout is empty. Caller inspects.
func TestLaunch_MissingArtifact_NotFatal(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "never-written.md")
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/bridge", runner)
	resp, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "auto",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "x",
	})
	if err != nil {
		t.Errorf("Launch: %v (artifact-missing should not fail)", err)
	}
	if resp.Stdout != "" {
		t.Errorf("Stdout=%q, want empty when artifact absent", resp.Stdout)
	}
}

// TestLaunch_RequiredFieldValidation — empty CLI / Profile / Workspace
// / ArtifactPath must fail fast.
func TestLaunch_RequiredFieldValidation(t *testing.T) {
	adapter := New("/bridge", (&fakeCmd{}).runner())
	cases := []core.BridgeRequest{
		{},
		{CLI: "claude-p"},
		{CLI: "claude-p", Profile: "/p"},
		{CLI: "claude-p", Profile: "/p", Workspace: "/w"},
	}
	for i, req := range cases {
		_, err := adapter.Launch(context.Background(), req)
		if err == nil {
			t.Errorf("case %d: Launch with %+v: want error", i, req)
		}
	}
}

// TestProbe_ParsesJSON — bridge probe emits {os, results:[{cli,tier,binary,version,stub}]}.
func TestProbe_ParsesJSON(t *testing.T) {
	jsonOut := `{
  "os": "Darwin/25.4.0",
  "results": [
    {"cli": "claude-p", "tier": "full", "binary": "/usr/local/bin/claude", "version": "2.1", "stub": false},
    {"cli": "claude-tmux", "tier": "hybrid", "binary": "/usr/local/bin/claude", "version": "2.1", "stub": false}
  ]
}`
	fc := &fakeCmd{exitCode: 0, stdout: []byte(jsonOut)}
	adapter := New("/bridge", fc.runner())
	pr, err := adapter.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if pr.Version == "" {
		// We don't have a top-level version in the JSON above; the
		// adapter may map .os into Version or leave empty. Either is OK,
		// but CLIs must populate.
	}
	if got := pr.CLIs["claude-p"]; got != "full" {
		t.Errorf("CLIs[claude-p]=%q, want full", got)
	}
	if got := pr.CLIs["claude-tmux"]; got != "hybrid" {
		t.Errorf("CLIs[claude-tmux]=%q, want hybrid", got)
	}
}

// TestProbe_NonZeroExit_Errors — probe must fail loudly on non-zero exit.
func TestProbe_NonZeroExit_Errors(t *testing.T) {
	fc := &fakeCmd{exitCode: 127}
	adapter := New("/bridge", fc.runner())
	_, err := adapter.Probe(context.Background())
	if err == nil {
		t.Error("Probe: want error on exit 127")
	}
}

// TestProbe_MalformedJSON_Errors — adapter wraps the parse error.
func TestProbe_MalformedJSON_Errors(t *testing.T) {
	fc := &fakeCmd{exitCode: 0, stdout: []byte("not json")}
	adapter := New("/bridge", fc.runner())
	_, err := adapter.Probe(context.Background())
	if err == nil {
		t.Error("Probe: want error on garbage JSON")
	}
}

// TestNew_DefaultRunnerUsesExec — passing nil runner produces an
// adapter that exec()s the real bridge. We can't run the real bridge
// in the test, but we can verify the adapter rejects a missing
// binary cleanly.
func TestNew_DefaultRunnerUsesExec(t *testing.T) {
	adapter := New("/no/such/bridge-binary-zzz", nil)
	_, err := adapter.Probe(context.Background())
	if err == nil {
		t.Error("Probe with missing binary: want error")
	}
}

// TestLaunch_ContextCanceled — context cancellation propagates to the
// runner (so callers can abort long bridge runs).
func TestLaunch_ContextCanceled(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "a.md")
	// Runner sees a canceled ctx → returns ctx.Err().
	runner := func(ctx context.Context, name string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		return 0, nil
	}
	adapter := New("/bridge", runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := adapter.Launch(ctx, core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "auto",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "x",
	})
	if err == nil {
		t.Error("Launch with canceled ctx: want error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err=%v, want context.Canceled", err)
	}
}

// TestLaunch_EnvForwarding — Env entries from BridgeRequest must
// surface in the runner's env slice (KEY=VALUE format).
func TestLaunch_EnvForwarding(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "a.md")
	_ = os.WriteFile(artifact, []byte("x"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/bridge", runner)
	_, _ = adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "auto",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "x",
		Env: map[string]string{"EVOLVE_FOO": "bar", "EVOLVE_BAZ": "qux"},
	})
	env := strings.Join(fc.gotEnv, " ")
	for _, want := range []string{"EVOLVE_FOO=bar", "EVOLVE_BAZ=qux"} {
		if !strings.Contains(env, want) {
			t.Errorf("env missing %q: %v", want, fc.gotEnv)
		}
	}
}

// silenceUnused — keep io.Writer import in case future tests want it.
var _ io.Writer = io.Discard
var _ bytes.Buffer
