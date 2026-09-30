package retro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func stalePath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "worktrees", "cycle-42824668-9999")
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("fixture path %q must not exist (stat err=%v)", p, err)
	}
	return p
}

func TestRetroWorktree_StaleNonExistentPathFallsBackToScratchCwd(t *testing.T) {
	projectRoot, workspace := t.TempDir(), t.TempDir()
	stale := stalePath(t)
	req := retroFailReq(projectRoot, workspace, stale, map[string]string{ipcenv.FleetKey: "1"})

	got := retroWorktree(req)
	if got == stale {
		t.Fatalf("retro passed the torn-down lane's stale worktree %q through verbatim — the bridge guard rejects it at isDir() (ExitBadFlags, stderr only) and the lane loses its retrospective entirely", got)
	}
	if got == "" {
		t.Fatalf("retro resolved no worktree despite owning a workspace (%q) — under fleet mode the bridge then refuses the launch with errWorktreeRequired: the same lost retrospective by a different exit code", workspace)
	}

	fi, err := os.Stat(got)
	if err != nil || !fi.IsDir() {
		t.Fatalf("resolved worktree %q is not an existing directory (%v) — a fabricated path is the exact shape this must never produce", got, err)
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

func TestRetroWorktree_FleetNeverEmitsANonExistentPath(t *testing.T) {
	stale := stalePath(t)
	req := retroFailReq(t.TempDir(), "", stale, map[string]string{ipcenv.FleetKey: "1"})

	got := retroWorktree(req)
	if got == "" {
		return
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Fatalf("retro emitted the non-existent path %q with no workspace to mint under (input was the stale %q) — every non-empty value retro returns must clear the bridge's isDir() guard", got, stale)
	}
}

func TestRetroWorktree_FleetProvisionedWorktreePassesThroughVerbatim(t *testing.T) {
	live := t.TempDir()
	req := retroFailReq(t.TempDir(), t.TempDir(), live, map[string]string{ipcenv.FleetKey: "1"})

	if got := retroWorktree(req); got != live {
		t.Fatalf("retroWorktree replaced the LIVE lane worktree %q with %q — a fallback that fires on an existing worktree strands every normal fleet retro in a repo-less scratch dir", live, got)
	}
}

func TestRetroWorktree_NonFleetStalePathPassesThroughVerbatim(t *testing.T) {
	stale := stalePath(t)
	req := retroFailReq(t.TempDir(), t.TempDir(), stale, map[string]string{ipcenv.FleetKey: "0"})

	if got := retroWorktree(req); got != stale {
		t.Fatalf("non-fleet dispatch rewrote the operator's designated worktree %q to %q — the fallback exists for the fleet guard's fail-closed window only", stale, got)
	}
}
