//go:build integration

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

// The cycle branch is pre-committed (worktree clean, branch ahead of main), so
// the pre-commit tree-SHA check never runs and the post-push check is the only
// guard between a wrong internalAuditBoundTreeSHA and a silently shipped drift.
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
