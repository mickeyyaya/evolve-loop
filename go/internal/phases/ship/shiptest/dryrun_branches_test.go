//go:build integration

package shiptest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/shiptest"
)

func TestShipFromWorktree_DryRun_LogsAndNoFFMerge(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "dry-run-branch")
	shiptest.MustWrite(t, filepath.Join(wt, "dry-change.txt"), "dry content\n")
	shiptest.RunGit(t, wt, "add", "dry-change.txt")
	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":99,"phase":"ship","active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")

	headBefore := strings.TrimSpace(shiptest.RunGitOut(t, repo, "rev-parse", "HEAD"))

	res, err := runShip(t, repo, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "dry: worktree dry run",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("DryRun worktree ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("DryRun should ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "[DRY-RUN]") {
		t.Errorf("missing DRY-RUN log lines: %v", res.Logs)
	}

	headAfter := strings.TrimSpace(shiptest.RunGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun must not ff-merge; HEAD moved %s → %s", headBefore, headAfter)
	}
}

func TestRun_DryRun_WritesJournal(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	shiptest.MustWrite(t, filepath.Join(repo, "dry.txt"), "something\n")
	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{
		Class:         ship.ClassCycle,
		CommitMessage: "dry: end-to-end",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("DryRun Run errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("DryRun Run want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if res.DryRunPath == "" {
		t.Fatal("DryRunPath must be set after a successful dry run")
	}
	if _, err := os.Stat(res.DryRunPath); err != nil {
		t.Fatalf("DryRunPath %s not on disk: %v", res.DryRunPath, err)
	}
}

func TestShipFromWorktree_CleanWorktreeAheadBranch_Merges(t *testing.T) {
	t.Parallel()
	repo := shiptest.MakeRepo(t)
	shiptest.AddRemote(t, repo)
	wt := shiptest.MakeWorktree(t, repo, "ahead-branch")

	shiptest.MustWrite(t, filepath.Join(wt, "already-committed.txt"), "already in wt\n")
	shiptest.RunGit(t, wt, "add", "-A")
	shiptest.RunGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-m", "pre-committed in worktree")

	shiptest.MustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":7,"phase":"ship","active_worktree":"`+wt+`"}`)
	shiptest.SeedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, ship.Options{Class: ship.ClassCycle, CommitMessage: "feat: ahead branch ship"})
	if err != nil {
		t.Fatalf("ahead-branch ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ship.ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "ff-merged ahead-branch into main") {
		t.Errorf("missing ff-merge log; got %v", res.Logs)
	}
	mainLog := shiptest.RunGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainLog, "already-committed.txt") {
		t.Errorf("ahead-branch commit not on main; HEAD log files: %q", mainLog)
	}
}
