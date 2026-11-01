//go:build integration

package shiptest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestShipFromWorktree_TreeSHABindingVerifiedLog(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "binding-test-branch")
	shiptest.MustWrite(t, filepath.Join(wt, "binding.txt"), "content\n")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":20,"active_worktree":"`+wt+`"}`)

	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: binding log test"})
	if err != nil {
		t.Fatalf("ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "ff-merged binding-test-branch into main") {
		t.Errorf("missing ff-merge log; got %v", res.Logs)
	}
}

func TestShipFromWorktree_WithAuditBoundTreeSHA_BindingLogged(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "treesha-branch")

	shiptest.MustWrite(t, filepath.Join(wt, "treesha.txt"), "bound content\n")
	shiptest.RunGit(t, wt, "add", "treesha.txt")
	shiptest.RunGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-m", "pre-ship commit in wt")

	wtTreeSHA := strings.TrimSpace(shiptest.RunGitOut(t, wt, "rev-parse", "HEAD^{tree}"))

	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":21,"active_worktree":"`+wt+`"}`)
	shiptest.SeedAuditWithBoundTree(t, repo, "PASS", wtTreeSHA)

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: bound tree ship"})
	if err != nil {
		t.Fatalf("bound tree ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "tree-SHA binding verified") {
		t.Errorf("missing tree-SHA binding verified log; got %v", res.Logs)
	}
}
