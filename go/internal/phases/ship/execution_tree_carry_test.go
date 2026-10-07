package ship

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
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

type laneCarriedOntoAPeer struct {
	lane carriedLane
	ref  string
}

func auditedLaneRebasedOntoAPeer(t *testing.T) laneCarriedOntoAPeer {
	t.Helper()
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
	return laneCarriedOntoAPeer{
		lane: carriedLane{repo: repo, worktree: wt, base0: base0, tree0: tree0, base1: base1, tree1: rev(wt, "write-tree"), cycle: 1715},
		ref:  mustHashFile(t, filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")),
	}
}

func (c laneCarriedOntoAPeer) ship(t *testing.T) (*RunResult, error) {
	t.Helper()
	opts := auditOpts(t, c.lane.repo)
	opts.ActiveWorktree = c.lane.worktree
	res := &RunResult{}
	return res, verifyAuditBinding(context.Background(), opts, res)
}

func carriedAndCleanedUp(t *testing.T, repo string, cycle int) {
	t.Helper()
	wt := filepath.Join(repo, ".evolve", "worktrees", fmt.Sprintf("cycle-cd3ae73e-%d", cycle))
	runGit(t, repo, "worktree", "add", "-q", "--detach", wt)
	base := strings.TrimSpace(runGitOut(t, wt, "rev-parse", "HEAD"))
	mustWrite(t, filepath.Join(wt, fmt.Sprintf("lane-%d.txt", cycle)), "an earlier lane's carried change\n")
	runGit(t, wt, "add", "-A")
	tree := strings.TrimSpace(runGitOut(t, wt, "write-tree"))
	diff, err := treedelta.Delta(context.Background(), testGit, wt, base, tree)
	if err != nil {
		t.Fatal(err)
	}
	earlier := carriedLane{repo: repo, worktree: wt, base0: base, tree0: tree, base1: base, tree1: tree, cycle: cycle}
	writeCarryOf(t, earlier, fmt.Sprintf("audit-of-cycle-%d", cycle), tree, tree, diff, diff)
	runGit(t, repo, "worktree", "remove", "--force", wt)
}

func TestVerifyAuditBinding_ShipsACarriedRebaseWithoutASecondAudit(t *testing.T) {
	c := auditedLaneRebasedOntoAPeer(t)
	writeCarry(t, c.lane, c.ref, c.lane.tree0)

	res, err := c.ship(t)

	if err != nil {
		t.Fatalf("verifyAuditBinding = %v; a peer landed before the audit bound, the lane rebased byte for byte, and its carry must ship on the verdict it earned (the live shape of cycles 1766, 1768 and 1772)", err)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "carry of cycle 1715, re-proven") {
		t.Errorf("logs = %q, want the binding to name the carry it accepted", logs)
	}
}

func TestVerifyAuditBinding_ShipsASecondCarryAfterTheFirstCarrysWorktreeIsGone(t *testing.T) {
	c := auditedLaneRebasedOntoAPeer(t)
	carriedAndCleanedUp(t, c.lane.repo, 1766)
	c.lane.cycle = 1782
	writeCarry(t, c.lane, c.ref, c.lane.tree0)

	res, err := c.ship(t)

	if err != nil {
		t.Fatalf("verifyAuditBinding = %v; cycle 1766 carried and its worktree was cleaned up, so cycle 1782's byte-identical carry must still ship (the live shape of cycles 1782 and 1810)", err)
	}
	if logs := strings.Join(res.Logs, "\n"); !strings.Contains(logs, "carry of cycle 1782, re-proven") {
		t.Errorf("logs = %q, want the binding to name the second carry", logs)
	}
}
