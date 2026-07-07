//go:build integration

// audit_bound_witness_integration_test.go — cycle-585 RED contract for
// preventiveAction #2 of the cycle-583 audit finding: "any post-audit tree
// mutation MUST be covered by a test that exercises the REAL push-integrity
// guard." Complements the static scan in audit_bound_witness_test.go, which
// pins WHO may write the field; this test pins WHAT HAPPENS when the value
// it holds doesn't match what was actually pushed to main — the exact
// failure mode a rebind (like the one cycle-583 rejected) would produce.
package ship

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestShipFromWorktree_PostPushGuard_FiresOnRebind simulates a post-merge
// rebind: the cycle branch already carries a real commit (worktree clean,
// branch ahead of main) so shipFromWorktree's pre-commit tree-SHA check
// (gitops.go:407-409) never runs — it only runs inside the "have an
// uncommitted worktree diff to stage" branch. That leaves the post-push
// check (gitops.go:493-497) as the ONLY guard standing between a wrong
// opts.internalAuditBoundTreeSHA and a silently-shipped tree-drift. Setting
// the field to a value that doesn't match the real committed tree must
// still surface CodeIntegrityTreeDrift from that post-push check, not a
// clean ship.
func TestShipFromWorktree_PostPushGuard_FiresOnRebind(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	seedAudit(t, repo, "PASS")

	wt := makeWorktree(t, repo, "cycle-1")
	mustWrite(t, filepath.Join(wt, "wt-change.txt"), "worktree change, already committed\n")
	runGit(t, wt, "add", "wt-change.txt")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "feat: pre-committed worktree change")

	opts := &Options{
		Class:                     ClassCycle,
		CommitMessage:             "feat: pre-committed worktree change",
		ProjectRoot:               repo,
		Runner:                    execRunner,
		Stdout:                    io.Discard,
		Stderr:                    io.Discard,
		internalAuditBoundTreeSHA: "0000000000000000000000000000000000dead",
	}
	res := &RunResult{}
	err := shipFromWorktree(context.Background(), opts, res, "main", wt)

	if err == nil {
		t.Fatalf("post-push guard must reject a rebound/mismatched audit-bound tree SHA, but ship succeeded silently — logs: %v", res.Logs)
	}
	var se *core.ShipError
	if !errors.As(err, &se) || se.Code != core.CodeIntegrityTreeDrift {
		t.Fatalf("want CodeIntegrityTreeDrift, got %v", err)
	}
	if se.Stage != core.StagePostShip {
		t.Fatalf("expected the POST-PUSH guard specifically (pre-commit check is skipped for an "+
			"already-committed, worktree-clean, branch-ahead scenario) to fire — got stage %q: %v", se.Stage, err)
	}
	if !strings.Contains(err.Error(), "committed tree SHA") {
		t.Fatalf("expected the post-push message shape (\"committed tree SHA\"), not the "+
			"pre-commit one (\"staged tree SHA\"), got: %v", err)
	}
}
