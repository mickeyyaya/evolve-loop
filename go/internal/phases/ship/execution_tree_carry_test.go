package ship

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestVerifyExecutionTree_AcceptsAReProvenCarry(t *testing.T) {
	l := rebasedLane(t)
	writeCarry(t, l, "audit-ref", l.tree0)
	res := &RunResult{}

	err := verifyExecutionTree(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), res, l.repo)

	if err != nil {
		t.Fatalf("verifyExecutionTree = %v; the audited change carried byte for byte onto a peer's base is the tree the audit's predicates bound (cycles 1766, 1768 and 1772 each paid a second audit for this)", err)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "carry of cycle 1715, re-proven") {
		t.Errorf("logs = %q, want the accepted drift to name the carry that explains it", logs)
	}
}

func TestVerifyExecutionTree_RefusesADriftNoRuleExplains(t *testing.T) {
	l := rebasedLane(t)

	err := verifyExecutionTree(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), &RunResult{}, l.repo)

	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "predicate execution tree-state")
}

func TestVerifyExecutionTree_RefusesACarryWhoseRecordNamesAnotherAudit(t *testing.T) {
	l := rebasedLane(t)
	writeCarry(t, l, "another-audit", l.tree0)

	err := verifyExecutionTree(context.Background(), boundTo(t, l, l.tree0, "audit-ref"), &RunResult{}, l.repo)

	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "predicate execution tree-state")
}

func TestVerifyExecutionTree_RefusesWhenTheTreeCannotBeTaken(t *testing.T) {
	opts := boundTo(t, rebasedLane(t), "", "audit-ref")

	err := verifyExecutionTree(context.Background(), opts, &RunResult{}, t.TempDir())

	wantShipErr(t, err, core.CodeAuditBindingTreeMismatch, core.ShipClassPrecondition, "predicate execution tree-state")
}

func TestVerifyAuditBinding_ShipsACarriedRebaseWithoutASecondAudit(t *testing.T) {
	repo := makeRepo(t)
	rev := func(dir string, args ...string) string { return strings.TrimSpace(runGitOut(t, dir, args...)) }
	base0 := rev(repo, "rev-parse", "HEAD")
	wt := tempRepoDir(t)
	runGit(t, repo, "worktree", "add", "-b", "cycle-1", wt)
	mustWrite(t, filepath.Join(wt, "lane.txt"), "the lane's audited change\n")
	mustWrite(t, filepath.Join(repo, "peer.txt"), "a peer landing\n")
	runGit(t, repo, "add", "peer.txt")
	runGit(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "peer")
	base1 := rev(repo, "rev-parse", "HEAD")
	statePath := filepath.Join(repo, ".evolve", "cycle-state.json")
	if err := writeStateMap(statePath, map[string]any{"active_worktree": wt}); err != nil {
		t.Fatal(err)
	}
	seedAudit(t, repo, "PASS")
	tree0 := rev(wt, "write-tree")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "lane")
	runGit(t, wt, "-c", "commit.gpgsign=false", "rebase", "-q", base1)
	runGit(t, wt, "reset", "-q", "--soft", base1)
	lane := carriedLane{repo: repo, base0: base0, tree0: tree0, base1: base1, tree1: rev(wt, "write-tree")}
	writeCarry(t, lane, mustHashFile(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")), tree0)
	opts := auditOpts(t, repo)
	opts.ActiveWorktree = wt
	res := &RunResult{}

	err := verifyAuditBinding(context.Background(), opts, res)

	if err != nil {
		t.Fatalf("verifyAuditBinding = %v; a peer landed before the audit bound, the lane rebased byte for byte, and its carry must ship on the verdict it earned (the live shape of cycles 1766, 1768 and 1772)", err)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "carry of cycle 1715, re-proven") {
		t.Errorf("logs = %q, want the binding to name the carry it accepted", logs)
	}
}
