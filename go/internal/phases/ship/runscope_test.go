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

// TestFindLatestAudit_RunIDNoMatch_RefusesUnstampedBind: runID set but every
// auditor entry is unstamped → hard integrity stop (NO_AUDITOR), never a bind.
// FLIPPED 2026-08-26 from _FallsBackToLatest: the old pin's "zero regression
// for pre-S4 ledgers" premise is dead (every current recorder stamps run_id),
// and cycle-1571 proved the fallback is the H3 fail-open hole — a FAILed
// cycle's ship bound cycle-1570's audit and returned AUDIT_BINDING_HEAD_MOVED
// instead of this run's FAIL, burning a re-audit slot; had the foreign entry's
// git_head matched HEAD, the FAILed cycle would have SHIPPED on a sibling's
// PASS. "This run produced no independent review" is an integrity stop, not a
// recoverable lookup miss.
func TestFindLatestAudit_RunIDNoMatch_RefusesUnstampedBind(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","git_head":"shaOld"}`+"\n"+
			`{"role":"auditor","kind":"agent_subprocess","git_head":"shaNew"}`+"\n")
	_, err := findLatestAudit(ledger, "Z")
	wantShipErr(t, err, core.CodeAuditBindingNoAuditor, core.ShipClassPrecondition, "independent review")
}

// TestFindLatestAudit_ForeignRunOnly_RefusesBind pins cycle-1571's exact H3
// shape: the only auditor entries belong to a DIFFERENT run (a sibling lane in
// the same HEAD window). Binding them lets one cycle's ship gate be satisfied
// by another cycle's audit; the error must name the refused foreign entry so
// an operator can see what would have been bound.
func TestFindLatestAudit_ForeignRunOnly_RefusesBind(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "ledger.jsonl")
	mustWrite(t, ledger,
		`{"role":"auditor","kind":"agent_subprocess","run_id":"A","git_head":"shaA"}`+"\n")
	_, err := findLatestAudit(ledger, "B")
	wantShipErr(t, err, core.CodeAuditBindingNoAuditor, core.ShipClassPrecondition, "run A")
}
