package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_ADispatchKeepsItsWritesToThePathsItMayWrite(t *testing.T) {
	t.Parallel()
	dir := fenceRepo(t)
	hooks := &fakeHooks{phase: "debugger", agent: "evolve-debugger", model: "opus", prompt: "body", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &mutatingBridge{}, Prompts: fakePromptsFS("evolve-debugger", "body")})
	req := core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: dir, WorktreeReadOnly: true, WorktreeWritablePaths: []string{"src/mat.go"}}

	resp, err := r.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got, _ := os.ReadFile(filepath.Join(dir, "src", "mat.go")); string(got) != "package src // MUTATED by a probe\n" {
		t.Errorf("the write to a path the dispatch may write was restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "zz_probe_test.go")); !os.IsNotExist(err) {
		t.Error("a file the dispatch may not write survived the fence")
	}
	var kept bool
	for _, d := range resp.Diagnostics {
		kept = kept || (strings.Contains(d.Message, "kept") && strings.Contains(d.Message, "src/mat.go"))
	}
	if !kept {
		t.Errorf("the kept write is not reported: %+v", resp.Diagnostics)
	}
}
