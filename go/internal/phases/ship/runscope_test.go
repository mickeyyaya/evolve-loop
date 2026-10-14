//go:build integration

package ship

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestReadActiveWorktree_PrefersRunJSON_OverGlobal(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-7")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"active_worktree":"/global/wrong-run"}`)
	mustWrite(t, filepath.Join(ws, core.RunStateFile), `{"active_worktree":"/runscoped/right-run"}`)

	opts := &Options{ProjectRoot: repo, WorkspacePath: ws}
	if got := readActiveWorktree(opts); got != "/runscoped/right-run" {
		t.Errorf("readActiveWorktree=%q, want the run-scoped run.json value (G3)", got)
	}
}

func TestReadActiveWorktree_FallsBackToGlobal_NoWorkspace(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"active_worktree":"/global/wt"}`)
	opts := &Options{ProjectRoot: repo}
	if got := readActiveWorktree(opts); got != "/global/wt" {
		t.Errorf("readActiveWorktree=%q, want global fallback", got)
	}
}

func TestReadActiveWorktree_FallsBackToGlobal_RunJSONAbsent(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	ws := filepath.Join(repo, ".evolve", "runs", "cycle-7")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"), `{"active_worktree":"/global/wt"}`)
	opts := &Options{ProjectRoot: repo, WorkspacePath: ws}
	if got := readActiveWorktree(opts); got != "/global/wt" {
		t.Errorf("readActiveWorktree=%q, want global fallback when run.json absent", got)
	}
}

func TestFindLatestAudit_PrefersThisRunsEntry(t *testing.T) {
	t.Parallel()
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

func TestFindLatestAudit_EmptyRunID_ReturnsLatest(t *testing.T) {
	t.Parallel()
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

func TestFindLatestAudit_RunIDNoMatch_RefusesUnstampedBind(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","git_head":"shaOld"}`+"\n"+
			`{"role":"auditor","kind":"agent_subprocess","git_head":"shaNew"}`+"\n")
	_, err := findLatestAudit(ledger, "Z")
	wantShipErr(t, err, core.CodeAuditBindingNoAuditor, core.ShipClassPrecondition, "independent review")
}

func TestFindLatestAudit_ForeignRunOnly_RefusesBind(t *testing.T) {
	t.Parallel()
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"A","git_head":"shaA"}`+"\n")
	_, err := findLatestAudit(ledger, "B")
	wantShipErr(t, err, core.CodeAuditBindingNoAuditor, core.ShipClassPrecondition, "run A")
}
