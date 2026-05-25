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

// TestLaunch_ExtraFlagsPassthroughSeparator pins the `--` separator
// that v12.1.0 was missing: bridge's launch parser uses a strict
// allowlist and rejects unknown flags fatally, so inner-CLI flags
// like --bare / --strict-mcp-config / --no-session-persistence
// (sourced from .evolve/profiles/<phase>.json:extra_flags) must
// arrive after `--` to be forwarded to the inner CLI rather than
// parsed by bridge.
//
// Production regression caught in cycle 106 (2026-05-25):
//
//	[bridge] launch: unknown flag: --bare
//
// — bridge tried to parse extra_flags as its own and aborted with
// EC_BAD_FLAGS=10. Without this test the next refactor could silently
// drop the separator again.
func TestLaunch_ExtraFlagsPassthroughSeparator(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "out.md")
	_ = os.WriteFile(artifact, []byte("ok"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/usr/local/bin/bridge", runner)
	_, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p.json", Model: "sonnet",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact,
		Agent: "scout", Cycle: 106,
		ExtraFlags: []string{
			"--bare",
			"--no-session-persistence",
			"--strict-mcp-config",
			"--exclude-dynamic-system-prompt-sections",
		},
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	// The argv MUST contain `--` exactly once, BEFORE the first
	// inner-CLI flag, after all bridge-native flags.
	args := fc.gotArgs
	sepIdx := -1
	for i, a := range args {
		if a == "--" {
			sepIdx = i
			break
		}
	}
	if sepIdx == -1 {
		t.Fatalf("argv missing `--` separator before ExtraFlags: %v", args)
	}
	// All bridge-native flags must appear BEFORE the separator.
	bridgeNative := []string{"launch", "--cli=claude-p", "--profile=/p.json",
		"--model=sonnet", "--artifact=" + artifact, "--cycle=106",
		"--agent=scout"}
	for _, want := range bridgeNative {
		found := -1
		for i := 0; i < sepIdx; i++ {
			if args[i] == want || strings.HasPrefix(args[i], want+"=") {
				found = i
				break
			}
		}
		if found == -1 {
			t.Errorf("bridge-native flag %q missing before separator (idx %d): %v",
				want, sepIdx, args)
		}
	}
	// All ExtraFlags must appear AFTER the separator, in original order.
	want := []string{"--bare", "--no-session-persistence",
		"--strict-mcp-config", "--exclude-dynamic-system-prompt-sections"}
	got := args[sepIdx+1:]
	if len(got) != len(want) {
		t.Fatalf("post-separator argv mismatch: got %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("post-separator[%d] = %q, want %q", i, got[i], w)
		}
	}
}

// TestLaunch_NoExtraFlagsNoSeparator — when ExtraFlags is empty/nil,
// the bare `--` separator MUST NOT appear (it would confuse the inner
// CLI with no flags to forward).
func TestLaunch_NoExtraFlagsNoSeparator(t *testing.T) {
	ws := t.TempDir()
	artifact := filepath.Join(ws, "out.md")
	_ = os.WriteFile(artifact, []byte("ok"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	runner := fc.runner()

	adapter := New("/usr/local/bin/bridge", runner)
	_, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p.json", Model: "sonnet",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact,
		Agent: "scout", Cycle: 106,
		// ExtraFlags omitted
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	for _, a := range fc.gotArgs {
		if a == "--" {
			t.Errorf("argv contains stray `--` separator with empty ExtraFlags: %v",
				fc.gotArgs)
		}
	}
}

// TestLaunch_PromptMaterializedAsFile — the prompt body must be
// written to a file under Workspace before the subprocess runs;
// --prompt-file=<path> appears in argv.
//
// As of v12.1 the bridge prepends a deterministic interactive-policy
// block (default recommended_or_first); this test asserts the body
// is present alongside the prefix, not bit-for-bit equality.
func TestLaunch_PromptMaterializedAsFile(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyEscalate)
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
	// With escalate policy the file must equal the body exactly.
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

// TestNew_DefaultBinaryAndRunner — empty binary/runner produce a
// usable adapter that falls back to "bridge" on PATH + execRunner.
func TestNew_DefaultBinaryAndRunner(t *testing.T) {
	a := New("", nil)
	if a == nil {
		t.Fatal("New(\"\", nil)=nil")
	}
	if a.binary != "bridge" {
		t.Errorf("default binary=%q, want 'bridge'", a.binary)
	}
}

// TestNewDefault_ResolvesProjectPath — wires the conventional
// tools/agent-bridge/bin/bridge path.
func TestNewDefault_ResolvesProjectPath(t *testing.T) {
	a := NewDefault("/repo")
	want := "/repo/tools/agent-bridge/bin/bridge"
	if a.binary != want {
		t.Errorf("binary=%q, want %q", a.binary, want)
	}
}

// TestExecRunner_ExitsWithCode — uses /usr/bin/false (exit 1) to drive
// the exec.ExitError branch of execRunner. POSIX universal.
func TestExecRunner_ExitsWithCode(t *testing.T) {
	if _, err := os.Stat("/usr/bin/false"); err != nil {
		t.Skip("/usr/bin/false not available")
	}
	var buf bytes.Buffer
	code, err := execRunner(context.Background(), "/usr/bin/false", nil, os.Environ(), nil, &buf, &buf)
	if err != nil {
		t.Errorf("execRunner /usr/bin/false: err=%v, want nil (ExitError mapped to exitCode)", err)
	}
	if code != 1 {
		t.Errorf("exitCode=%d, want 1", code)
	}
}

// TestExecRunner_BinaryMissing — non-existent binary → err non-nil.
func TestExecRunner_BinaryMissing(t *testing.T) {
	var buf bytes.Buffer
	_, err := execRunner(context.Background(), "/no/such/binary/xyz", nil, os.Environ(), nil, &buf, &buf)
	if err == nil {
		t.Error("execRunner with missing binary: want error")
	}
}

// TestLaunch_WorkspaceMkdirFails — when Workspace path can't be
// created (parent is a file), Launch reports the error.
func TestLaunch_WorkspaceMkdirFails(t *testing.T) {
	tmp := t.TempDir()
	parent := filepath.Join(tmp, "blocker")
	if err := os.WriteFile(parent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(parent, "child") // parent is a file, child can't be mkdir'd
	artifact := filepath.Join(ws, "a.md")
	adapter := New("/bridge", (&fakeCmd{}).runner())
	_, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "auto",
		Prompt: "x", Workspace: ws, ArtifactPath: artifact, Agent: "x",
	})
	if err == nil {
		t.Error("Launch with un-creatable workspace: want error")
	}
}

// TestTruncate_LongString — direct test of truncate helper to hit the
// > n branch.
func TestTruncate_LongString(t *testing.T) {
	got := truncate("abcdefghij", 5)
	if got != "abcde…" {
		t.Errorf("truncate=%q, want abcde…", got)
	}
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("truncate short=%q, want abc", got)
	}
}

// TestNonEmpty_BothBranches — direct test of nonEmpty helper.
func TestNonEmpty_BothBranches(t *testing.T) {
	if got := nonEmpty("", "fb"); got != "fb" {
		t.Errorf("nonEmpty(\"\")=%q, want fb", got)
	}
	if got := nonEmpty("real", "fb"); got != "real" {
		t.Errorf("nonEmpty(real)=%q, want real", got)
	}
}

// --- v12.1 Capability 3: interactive-policy injection tests ---

// readPromptFile pulls the materialized prompt file from a recorded
// argv slice. Returns the file contents and a cleanup-friendly error.
func readPromptFile(t *testing.T, args []string) string {
	t.Helper()
	for _, a := range args {
		if strings.HasPrefix(a, "--prompt-file=") {
			path := strings.TrimPrefix(a, "--prompt-file=")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read prompt file: %v", err)
			}
			return string(b)
		}
	}
	t.Fatalf("argv missing --prompt-file: %v", args)
	return ""
}

// runOnce launches the adapter against a fakeCmd that captures argv and
// returns the prompt-file contents.
func runOnce(t *testing.T, agent, prompt string, env map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	artifact := filepath.Join(ws, "out.md")
	_ = os.WriteFile(artifact, []byte("ok"), 0o644)
	fc := &fakeCmd{exitCode: 0}
	adapter := New("/bridge", fc.runner())
	_, err := adapter.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "/p", Model: "auto",
		Prompt: prompt, Workspace: ws, ArtifactPath: artifact, Agent: agent,
		Env: env,
	})
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	return readPromptFile(t, fc.gotArgs)
}

// TestLaunch_DefaultPolicy_InjectsRecommendedOrFirstPrefix — with no
// EVOLVE_INTERACTIVE_POLICY env set, the prompt file is prefixed with
// the recommended-or-first policy block. Default-on autonomy posture.
func TestLaunch_DefaultPolicy_InjectsRecommendedOrFirstPrefix(t *testing.T) {
	// Explicitly clear both global and per-agent overrides so the
	// test is hermetic against an inherited operator shell.
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	t.Setenv("EVOLVE_SCOUT_INTERACTIVE_POLICY", "")
	body := runOnce(t, "scout", "scout prompt body", nil)
	if !strings.HasPrefix(body, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("prompt missing recommended-or-first prefix; got first 80 chars: %q", truncate(body, 80))
	}
	if !strings.Contains(body, "scout prompt body") {
		t.Errorf("prompt missing original body after prefix")
	}
}

// TestLaunch_NoPolicyPrefix_WhenEscalateExplicit — operator opts out of
// auto-resolution; legacy fail-loud behavior is preserved.
func TestLaunch_NoPolicyPrefix_WhenEscalateExplicit(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyEscalate)
	body := runOnce(t, "builder", "builder body", nil)
	if strings.Contains(body, "Subagent Interactive Policy") {
		t.Errorf("escalate policy must not inject a block; got first 120 chars: %q", truncate(body, 120))
	}
	if body != "builder body" {
		t.Errorf("body=%q, want %q (no prefix, no suffix)", body, "builder body")
	}
}

// TestLaunch_AutoYesPolicy_InjectsAlternatePrefix — auto_yes injects
// the binary-prompt variant of the policy block.
func TestLaunch_AutoYesPolicy_InjectsAlternatePrefix(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	body := runOnce(t, "auditor", "auditor body", nil)
	if !strings.HasPrefix(body, "## Subagent Interactive Policy (auto_yes)") {
		t.Errorf("auto_yes policy must inject auto_yes block; got first 80 chars: %q", truncate(body, 80))
	}
	if !strings.Contains(body, "auditor body") {
		t.Errorf("prompt missing original body after prefix")
	}
}

// TestResolvePolicy_PrecedenceOrder — per-agent reqEnv beats process
// env which beats global env which beats default.
func TestResolvePolicy_PrecedenceOrder(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	t.Setenv("EVOLVE_SCOUT_INTERACTIVE_POLICY", PolicyEscalate)

	// per-agent process env beats global process env
	if got := resolvePolicy("scout", nil); got != PolicyEscalate {
		t.Errorf("per-agent env should win: got=%q want=%q", got, PolicyEscalate)
	}
	// per-agent reqEnv beats per-agent process env
	if got := resolvePolicy("scout", map[string]string{
		"EVOLVE_SCOUT_INTERACTIVE_POLICY": PolicyRecommendedOrFirst,
	}); got != PolicyRecommendedOrFirst {
		t.Errorf("reqEnv per-agent should win: got=%q want=%q", got, PolicyRecommendedOrFirst)
	}
	// global reqEnv beats global process env when per-agent absent
	if got := resolvePolicy("builder", map[string]string{
		"EVOLVE_INTERACTIVE_POLICY": PolicyRecommendedOrFirst,
	}); got != PolicyRecommendedOrFirst {
		t.Errorf("reqEnv global should win: got=%q want=%q", got, PolicyRecommendedOrFirst)
	}
	// fall through to global process env when no overrides
	if got := resolvePolicy("builder", nil); got != PolicyAutoYes {
		t.Errorf("global env should be used: got=%q want=%q", got, PolicyAutoYes)
	}
}

// TestResolvePolicy_DefaultWhenAllUnset — empty env returns the
// default policy.
func TestResolvePolicy_DefaultWhenAllUnset(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	t.Setenv("EVOLVE_BUILDER_INTERACTIVE_POLICY", "")
	if got := resolvePolicy("builder", nil); got != PolicyRecommendedOrFirst {
		t.Errorf("default policy got=%q want=%q", got, PolicyRecommendedOrFirst)
	}
}

// TestResolvePolicy_EmptyAgent_FallsThroughToGlobal — when agent name
// is empty, per-agent lookup is skipped entirely.
func TestResolvePolicy_EmptyAgent_FallsThroughToGlobal(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	if got := resolvePolicy("", nil); got != PolicyAutoYes {
		t.Errorf("empty agent should fall through to global: got=%q want=%q", got, PolicyAutoYes)
	}
}

// TestPerAgentPolicyEnv_HyphenToUnderscore — hyphenated agent names
// like "tdd-engineer" map to underscored env keys.
func TestPerAgentPolicyEnv_HyphenToUnderscore(t *testing.T) {
	cases := map[string]string{
		"scout":         "EVOLVE_SCOUT_INTERACTIVE_POLICY",
		"builder":       "EVOLVE_BUILDER_INTERACTIVE_POLICY",
		"tdd-engineer":  "EVOLVE_TDD_ENGINEER_INTERACTIVE_POLICY",
		"plan-reviewer": "EVOLVE_PLAN_REVIEWER_INTERACTIVE_POLICY",
	}
	for agent, want := range cases {
		if got := perAgentPolicyEnv(agent); got != want {
			t.Errorf("perAgentPolicyEnv(%q)=%q, want %q", agent, got, want)
		}
	}
}

// TestInjectPolicyPrefix_UnknownValueDefaultsToRecommendedOrFirst — a
// typo in EVOLVE_INTERACTIVE_POLICY should NOT break autonomy: the
// bridge silently falls back to recommended-or-first.
func TestInjectPolicyPrefix_UnknownValueDefaultsToRecommendedOrFirst(t *testing.T) {
	got := injectPolicyPrefix("body", "no-such-policy")
	if !strings.HasPrefix(got, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("unknown policy should default to recommended_or_first; got first 80 chars: %q", truncate(got, 80))
	}
}

// TestInjectPolicyPrefix_EscalateReturnsBodyUnchanged — direct unit
// test of the helper to confirm zero-allocation pass-through for
// operators who opt out.
func TestInjectPolicyPrefix_EscalateReturnsBodyUnchanged(t *testing.T) {
	if got := injectPolicyPrefix("body", PolicyEscalate); got != "body" {
		t.Errorf("escalate should pass through unchanged; got=%q", got)
	}
}

// TestLaunch_PerAgentEnvOverrides_GlobalDefault — operator pins the
// scout agent to escalate while every other agent stays on the default
// recommended_or_first. Validates the per-agent override seam end-to-end.
func TestLaunch_PerAgentEnvOverrides_GlobalDefault(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	t.Setenv("EVOLVE_SCOUT_INTERACTIVE_POLICY", PolicyEscalate)

	scoutBody := runOnce(t, "scout", "scout body", nil)
	if scoutBody != "scout body" {
		t.Errorf("scout per-agent escalate not honored; got %q", truncate(scoutBody, 120))
	}

	builderBody := runOnce(t, "builder", "builder body", nil)
	if !strings.HasPrefix(builderBody, "## Subagent Interactive Policy (recommended_or_first)") {
		t.Errorf("builder should still get default block; got first 80 chars: %q", truncate(builderBody, 80))
	}
}

// TestLaunch_ReqEnvOverridesProcessEnv — req.Env beats os.Getenv. This
// lets the orchestrator pin a policy for a single phase without
// mutating its own environment.
func TestLaunch_ReqEnvOverridesProcessEnv(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", PolicyAutoYes)
	body := runOnce(t, "builder", "builder body", map[string]string{
		"EVOLVE_INTERACTIVE_POLICY": PolicyEscalate,
	})
	if body != "builder body" {
		t.Errorf("reqEnv should override process env; got %q", truncate(body, 120))
	}
}

// TestLaunch_PolicyBlockStableAcrossRuns — two launches of the same
// agent with the same policy produce byte-identical prefixes (cache
// stability, per the v12.1 plan's prompt-cache constraint).
func TestLaunch_PolicyBlockStableAcrossRuns(t *testing.T) {
	t.Setenv("EVOLVE_INTERACTIVE_POLICY", "")
	body1 := runOnce(t, "scout", "x", nil)
	body2 := runOnce(t, "scout", "y", nil)
	prefix1 := strings.TrimSuffix(body1, "x")
	prefix2 := strings.TrimSuffix(body2, "y")
	if prefix1 != prefix2 {
		t.Errorf("policy prefix is not stable across runs (cache invalidation risk)\n  run1: %q\n  run2: %q",
			truncate(prefix1, 100), truncate(prefix2, 100))
	}
}

// silenceUnused — keep io.Writer import in case future tests want it.
var _ io.Writer = io.Discard
var _ bytes.Buffer
