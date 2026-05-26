package bridge

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// coverage_test.go — closes the last adapter branches: New defaults + the
// default engineFactory closure, execRunner success + missing-binary, the
// prompt-write error, and the optional-flag argv appends.

func TestAdapter_New_DefaultsAndFactory(t *testing.T) {
	a := New("", nil) // binary "" → "bridge"; runner nil → execRunner
	if a.binary != "bridge" {
		t.Fatalf("default binary = %q, want bridge", a.binary)
	}
	if a.runner == nil {
		t.Fatal("default runner should be execRunner")
	}
	// Invoke the default engineFactory closure directly (covers its body
	// without running a real in-process engine).
	if eng := a.engineFactory(map[string]string{"K": "V"}); eng == nil {
		t.Fatal("default engineFactory should build an in-process bridge.Engine")
	}
}

func TestAdapter_ExecRunner_Success(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("no `true` binary; skipping execRunner success coverage")
	}
	ws := t.TempDir()
	a := New("true", nil) // real execRunner over `true` (exits 0, ignores args)
	if _, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "p", Workspace: ws,
		ArtifactPath: filepath.Join(ws, "a.md"), Prompt: "x",
	}); err != nil {
		t.Fatalf("Launch over `true` should succeed: %v", err)
	}
}

func TestAdapter_ExecRunner_MissingBinary(t *testing.T) {
	ws := t.TempDir()
	a := New("/no/such/bridge-binary-xyz", nil)
	if _, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "p", Workspace: ws,
		ArtifactPath: filepath.Join(ws, "a.md"), Prompt: "x",
	}); err == nil {
		t.Fatal("Launch over a missing bridge binary should error (execRunner returns -1)")
	}
}

func TestAdapter_Launch_PromptWriteError(t *testing.T) {
	ws := t.TempDir()
	// promptFile is <agent>-prompt.txt; Agent "" → "agent". Make it a dir
	// so os.WriteFile fails.
	if err := os.Mkdir(filepath.Join(ws, "agent-prompt.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := New("bridge", func(context.Context, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		t.Fatal("runner should not be reached when prompt write fails")
		return 0, nil
	})
	if _, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "p", Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"), Prompt: "x",
	}); err == nil {
		t.Fatal("Launch should fail when the prompt file can't be written")
	}
}

func TestAdapter_Launch_OptionalFlagsAppended(t *testing.T) {
	ws := t.TempDir()
	var gotArgs []string
	a := New("bridge", func(_ context.Context, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		gotArgs = args
		return 0, nil
	})
	if _, err := a.Launch(context.Background(), core.BridgeRequest{
		CLI: "claude-p", Profile: "p", Model: "auto", Prompt: "x",
		Workspace: ws, ArtifactPath: filepath.Join(ws, "a.md"),
		Cycle: 3, Agent: "scout", Worktree: ws, ExtraFlags: []string{"--bare"},
	}); err != nil {
		t.Fatalf("Launch err: %v", err)
	}
	joined := strings.Join(gotArgs, " ")
	for _, want := range []string{"--cycle=3", "--agent=scout", "--worktree=" + ws, "--", "--bare"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv missing %q; args=%v", want, gotArgs)
		}
	}
}
