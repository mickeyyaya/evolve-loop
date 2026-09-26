package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func readRunnerSource(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	var source strings.Builder
	for _, name := range []string{"runner.go", "dispatch.go"} {
		src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source.Write(src)
	}
	return source.String()
}

func TestRunnerDispatch_NoInlineFallbackLoop(t *testing.T) {
	src := readRunnerSource(t)
	if strings.Contains(src, "for i, candidateCLI := range plan.Candidates") {
		t.Errorf("runner.go still contains the inline WS-G1 fallback loop — delegate to llmroute.Dispatch instead ([[never_duplicate_centralize_via_design_patterns]])")
	}
}

func TestRunnerDispatch_CallsLlmrouteDispatch(t *testing.T) {
	src := readRunnerSource(t)
	if !strings.Contains(src, "llmroute.DispatchTiered(") {
		t.Errorf("runner.go does not call llmroute.DispatchTiered — the tier+CLI fallback walk must live in the shared llmroute package, not a hand-rolled copy in the runner")
	}

	hooks := &fakeHooks{
		phase: "auditor", agent: "evolve-auditor", model: "sonnet",
		prompt: "x", verdict: core.VerdictPASS, nextPhase: "ship",
	}
	sb := &scriptedBridge{
		responses: map[string]scriptedResp{
			"codex-tmux": {
				resp: core.BridgeResponse{ExitCode: 80, Stderr: "REPL boot timeout"},
				err:  errors.New("bridge: launch exit=80"),
			},
			"claude-tmux": {}, // empty = success
		},
	}
	root := writeFallbackProfile(t, "evolve-auditor", "codex-tmux", []string{"claude-tmux"})
	r := New(Options{
		Hooks:   hooks,
		Bridge:  sb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
	})

	if _, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	}); err != nil {
		t.Fatalf("Run: expected the fallback to still succeed post-dedup, got err=%v", err)
	}
	want := []string{"codex-tmux", "claude-tmux"}
	if len(sb.calls) != len(want) || sb.calls[0] != want[0] || sb.calls[1] != want[1] {
		t.Errorf("Run: dispatched %v, want %v (chain behavior preserved through the Dispatch delegation)", sb.calls, want)
	}
}
