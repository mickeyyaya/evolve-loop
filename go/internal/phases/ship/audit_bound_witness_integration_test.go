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

func TestShipFromWorktree_ARebindIsRefusedBeforeThePush(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	runGit(t, repo, "push", "-q", "origin", "main")
	published := remoteHeadSHA(t, repo)
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
	if !strings.Contains(err.Error(), "INTEGRITY BREACH (pre-push)") || !strings.Contains(err.Error(), "landing commit tree SHA") {
		t.Fatalf("want the pre-push landing check, the guard for an already-committed, branch-ahead lane that no pre-commit check sees: %v", err)
	}
	if got := remoteHeadSHA(t, repo); got != published {
		t.Errorf("origin moved to %s: drift from the audit-bound tree is refused before any push", got)
	}
}
