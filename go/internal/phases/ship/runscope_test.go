//go:build integration

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

// TestFindLatestAudit_PrefersThisRunsEntry pins ADR-0049 S4 / gap G5: with a
// runID set, ship binds to THIS run's auditor entry, not a concurrent run's
// later one. Ledger: run B auditor (older) then run A auditor (newer/latest);
// findLatestAudit(ledger,"B") must return B, not the latest A. RED before the
// run-filter (returns A), GREEN after.
func TestFindLatestAudit_PrefersThisRunsEntry(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","exit_code":0,"run_id":"B","git_head":"shaB"}`+"\n"+
			`{"role":"auditor","kind":"agent_subprocess","exit_code":0,"run_id":"A","git_head":"shaA"}`+"\n")
	e, err := findLatestAudit(ledger, "B")
	if err != nil {
		t.Fatalf("findLatestAudit: %v", err)
	}
	if e.RunID != "B" || e.GitHEAD != "shaB" {
		t.Errorf("bound run_id=%q git_head=%q, want run B (G5: must not bind concurrent run A's latest entry)", e.RunID, e.GitHEAD)
	}
}

// TestFindLatestAudit_EmptyRunID_ReturnsLatest: standalone (runID=="") keeps
// binding the latest auditor entry overall (pre-S4 behavior).
func TestFindLatestAudit_EmptyRunID_ReturnsLatest(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"B","git_head":"shaB"}`+"\n"+
			`{"role":"auditor","kind":"agent_subprocess","run_id":"A","git_head":"shaA"}`+"\n")
	e, err := findLatestAudit(ledger, "")
	if err != nil {
		t.Fatalf("findLatestAudit: %v", err)
	}
	if e.GitHEAD != "shaA" {
		t.Errorf("empty runID got git_head=%q, want latest shaA", e.GitHEAD)
	}
}

// TestFindLatestAudit_RunIDNoMatch_FallsBackToLatest: runID set but no entry
// carries it (legacy/unstamped) → fall back to the latest auditor entry (zero
// regression for pre-S4 ledgers).
func TestFindLatestAudit_RunIDNoMatch_FallsBackToLatest(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","git_head":"shaOld"}`+"\n"+
			`{"role":"auditor","kind":"agent_subprocess","git_head":"shaNew"}`+"\n")
	e, err := findLatestAudit(ledger, "Z")
	if err != nil {
		t.Fatalf("findLatestAudit: %v", err)
	}
	if e.GitHEAD != "shaNew" {
		t.Errorf("no-match fallback got git_head=%q, want latest shaNew", e.GitHEAD)
	}
}
