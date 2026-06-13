package subagent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
)

// TestBuildWorkerRecursionCommand proves a fan-out worker recurses into the
// SAME evolve binary via the bridge dispatch path (`subagent run`), never the
// in-process tool — and threads the recursion depth + clears the host marker.
func TestBuildWorkerRecursionCommand(t *testing.T) {
	cmd := buildWorkerRecursionCommand("/path/to/evolve", "auditor", "security", 7, 2, "/ws", "/tmp/p.txt", "ptok-worker-security")

	// Re-enters the bridge dispatch path for the correct worker name, with the
	// binary + workspace shell-quoted.
	if !strings.Contains(cmd, "'/path/to/evolve' subagent run auditor-worker-security 7 '/ws'") {
		t.Errorf("command does not re-enter `subagent run` for the worker: %q", cmd)
	}
	// Threads the prompt (quoted) + the incremented recursion depth.
	if !strings.Contains(cmd, "PROMPT_FILE_OVERRIDE='/tmp/p.txt'") {
		t.Errorf("command missing quoted PROMPT_FILE_OVERRIDE: %q", cmd)
	}
	if !strings.Contains(cmd, "EVOLVE_DISPATCH_DEPTH=2") {
		t.Errorf("command missing threaded recursion depth EVOLVE_DISPATCH_DEPTH=2: %q", cmd)
	}
	// A recursive child is NEVER the host: the host marker must be cleared so
	// DetectNested stays true at every depth (no inner sandbox wrap → no EPERM).
	if !strings.Contains(cmd, "CLAUDECODE_TYPE=") {
		t.Errorf("command must clear CLAUDECODE_TYPE for the child: %q", cmd)
	}
	if strings.Contains(cmd, "CLAUDECODE_TYPE=host") {
		t.Errorf("command must NOT mark the recursive child as host: %q", cmd)
	}
}

// TestBuildWorkerRecursionCommand_QuotesPathsWithSpaces proves paths containing
// spaces (or shell metacharacters) are quoted so the /bin/sh -c worker command
// does not word-split or inject.
func TestBuildWorkerRecursionCommand_QuotesPathsWithSpaces(t *testing.T) {
	cmd := buildWorkerRecursionCommand("/My Apps/evolve", "scout", "docs", 3, 1, "/home/u/my run/ws", "/tmp/a b.txt", "ptok-worker-docs")
	for _, want := range []string{
		"'/My Apps/evolve'",
		"'/home/u/my run/ws'",
		"PROMPT_FILE_OVERRIDE='/tmp/a b.txt'",
	} {
		if !strings.Contains(cmd, want) {
			t.Errorf("command missing quoted segment %q: %q", want, cmd)
		}
	}
}

// TestEnforceDispatchDepth_Boundary pins the self-depth fence exactly:
// depth==cap is allowed, depth==cap+1 is rejected.
func TestEnforceDispatchDepth_Boundary(t *testing.T) {
	if err := enforceDispatchDepth(maxDispatchDepth); err != nil {
		t.Errorf("depth==cap (%d) must be allowed, got %v", maxDispatchDepth, err)
	}
	if err := enforceDispatchDepth(maxDispatchDepth + 1); !errors.Is(err, ErrRecursionDepthExceeded) {
		t.Errorf("depth==cap+1 must be rejected, got %v", err)
	}
}

// TestEnforceChildDispatchDepth_Boundary pins the child fence: fanning out at
// parentDepth==cap-1 is allowed (child==cap), but at parentDepth==cap it is
// rejected fast (child==cap+1 would exceed) — the fail-fast the comment promises.
func TestEnforceChildDispatchDepth_Boundary(t *testing.T) {
	if err := enforceChildDispatchDepth(maxDispatchDepth - 1); err != nil {
		t.Errorf("parentDepth==cap-1 (child==cap) must be allowed, got %v", err)
	}
	if err := enforceChildDispatchDepth(maxDispatchDepth); !errors.Is(err, ErrRecursionDepthExceeded) {
		t.Errorf("parentDepth==cap (child==cap+1) must be rejected fast, got %v", err)
	}
}

func TestReadDispatchDepth(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0}, {"0", 0}, {"2", 2}, {" 3 ", 3}, {"-1", 0}, {"abc", 0},
	}
	for _, c := range cases {
		got := ReadDispatchDepth(func(k string) string {
			if k == "EVOLVE_DISPATCH_DEPTH" {
				return c.in
			}
			return ""
		})
		if got != c.want {
			t.Errorf("ReadDispatchDepth(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestRun_RecursionDepthCap proves a dispatch deeper than the cap is a hard
// error — bounding runaway nested fan-out.
func TestRun_RecursionDepthCap(t *testing.T) {
	tmp := t.TempDir()
	_, err := Run(context.Background(), RunRequest{
		Agent:         "scout",
		Cycle:         0,
		WorkspacePath: tmp,
		ProjectRoot:   tmp,
		PromptReader:  strings.NewReader("hi"),
		DispatchDepth: maxDispatchDepth + 1,
	}, runHappyOpts(t))
	if !errors.Is(err, ErrRecursionDepthExceeded) {
		t.Fatalf("expected ErrRecursionDepthExceeded at depth %d, got %v", maxDispatchDepth+1, err)
	}
}

// TestRecursionStaysNested_NoInnerWrap documents the sandbox-coherence
// invariant: at any recursion depth the worker runs inside an outer Claude
// (CLAUDECODE set) and is not marked host, so DetectNested=true and the inner
// sandbox is never applied — the exact condition that avoids the macOS
// sandbox_apply() EPERM REPL hang under nesting.
func TestRecursionStaysNested_NoInnerWrap(t *testing.T) {
	childEnv := map[string]string{"CLAUDECODE": "1"} // host marker cleared by buildWorkerRecursionCommand
	nested := sandbox.DetectNested(func(k string) string { return childEnv[k] })
	if !nested {
		t.Fatal("recursive child must be detected as nested")
	}
	wrap, _ := sandbox.ShouldWrap(nested, sandbox.ProbeResult{OS: "darwin", Available: true})
	if wrap {
		t.Error("recursive child must NOT be inner-sandbox wrapped (would EPERM-hang the REPL)")
	}
}
