package retro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestRetroWorktree_FleetScratchCwdSatisfiesBridgeGuardPredicate(t *testing.T) {
	projectRoot, workspace := t.TempDir(), t.TempDir()
	req := retroFailReq(projectRoot, workspace, "", map[string]string{ipcenv.FleetKey: "1"})

	got := retroWorktree(req)
	if got == "" {
		t.Fatal("retro resolved no worktree under a fleet dispatch with an owned workspace — the bridge then refuses the launch (errWorktreeRequired) and the lane loses its retrospective entirely: a failure in the failure-handler")
	}

	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("resolved worktree %q is not an existing directory (%v) — the fleet guard rejects it at isDir() and a fabricated path is exactly the shape this must never produce", got, err)
	}

	if got == projectRoot || strings.HasPrefix(got, projectRoot+string(filepath.Separator)) {
		t.Errorf("resolved worktree %q is inside the shared main tree — worktree is the write-authority predicate (refuted PR #400)", got)
	}
	if cwd, cerr := os.Getwd(); cerr == nil && got == cwd {
		t.Errorf("resolved worktree is the dispatching process cwd (%q) — the exact leak the fleet guard exists to close", got)
	}
	if !strings.HasPrefix(got, workspace+string(filepath.Separator)) {
		t.Errorf("resolved worktree %q is not under the workspace retro owns (%q) — a disposable cwd must live where the lane already has write authority", got, workspace)
	}
}
