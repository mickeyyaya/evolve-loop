package ship

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestReadActiveWorktree_PrefersRunJSON_OverGlobal pins ADR-0049 S3 / gap G3:
// when a run workspace is set, ship reads active_worktree from the per-run
// run.json mirror, NOT the host-global cycle-state.json — so a concurrent
// cycle's global write can't make ship integrate the WRONG run's worktree.
// RED before readActiveWorktree consults cycleStateFile (returns the global
// value), GREEN after.
func TestReadActiveWorktree_PrefersRunJSON_OverGlobal(t *testing.T) {
	repo := makeRepo(t)
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-7")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"active_worktree":"/global/wrong-run"}`)
	mustWrite(t, filepath.Join(ws, core.RunStateFile), `{"active_worktree":"/runscoped/right-run"}`)

	opts := &Options{ProjectRoot: repo, WorkspacePath: ws}
	if got := readActiveWorktree(opts); got != "/runscoped/right-run" {
		t.Errorf("readActiveWorktree=%q, want the run-scoped run.json value (G3)", got)
	}
}

// TestReadActiveWorktree_FallsBackToGlobal_NoWorkspace: standalone `evolve ship`
// (no WorkspacePath) keeps reading the host-global cycle-state.json.
func TestReadActiveWorktree_FallsBackToGlobal_NoWorkspace(t *testing.T) {
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"active_worktree":"/global/wt"}`)
	opts := &Options{ProjectRoot: repo} // no WorkspacePath
	if got := readActiveWorktree(opts); got != "/global/wt" {
		t.Errorf("readActiveWorktree=%q, want global fallback", got)
	}
}

// TestReadActiveWorktree_FallsBackToGlobal_RunJSONAbsent: WorkspacePath set but
// the mirror not yet written → fall back to the global file (current behavior;
// fail-safe, never an empty worktree from a missing mirror).
func TestReadActiveWorktree_FallsBackToGlobal_RunJSONAbsent(t *testing.T) {
	repo := makeRepo(t)
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-7") // no run.json written
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"active_worktree":"/global/wt"}`)
	opts := &Options{ProjectRoot: repo, WorkspacePath: ws}
	if got := readActiveWorktree(opts); got != "/global/wt" {
		t.Errorf("readActiveWorktree=%q, want global fallback when run.json absent", got)
	}
}
