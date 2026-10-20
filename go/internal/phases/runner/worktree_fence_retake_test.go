package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRun_ARetakenSnapshotNamesTheFirstFailure(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	dir := fenceRepo(t)
	bin := t.TempDir()
	count := filepath.Join(bin, "count")
	script := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = write-tree ] && [ ! -f %[1]q ]; then : > %[1]q; echo 'shim: transient write-tree failure' >&2; exit 3; fi\nexec %[2]q \"$@\"\n", count, realGit)
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	hooks := &fakeHooks{phase: "audit", agent: "evolve-audit", model: "sonnet", prompt: "body", verdict: core.VerdictPASS}
	r := New(Options{Hooks: hooks, Bridge: &mutatingBridge{}, Prompts: fakePromptsFS("evolve-audit", "body")})

	resp, err := r.Run(context.Background(), core.PhaseRequest{ProjectRoot: t.TempDir(), Workspace: t.TempDir(), Worktree: dir, WorktreeReadOnly: true})

	var retaken, restored bool
	for _, d := range resp.Diagnostics {
		retaken = retaken || strings.Contains(d.Message, "snapshot retaken after:") && strings.Contains(d.Message, "transient write-tree failure")
		restored = restored || strings.Contains(d.Message, "restored to the dispatched tree")
	}
	if err != nil || !retaken || !restored {
		t.Errorf("Run err %v, diagnostics %+v; want the retaken warning that names the first failure, and the fence still restoring the write", err, resp.Diagnostics)
	}
}
