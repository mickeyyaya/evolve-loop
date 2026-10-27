//go:build integration

package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipDirect_DryRun_LogsAndNoCommit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "change.txt"), "dry-run change\n")
	runGit(t, repo, "add", "change.txt")

	headBefore := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	res := &RunResult{}
	opts := &Options{
		Class:         ClassManual,
		CommitMessage: "dry: should not commit",
		ProjectRoot:   repo,
		DryRun:        true,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	err := shipDirect(context.Background(), opts, res, "main")
	if err != nil {
		t.Fatalf("DryRun shipDirect errored: %v", err)
	}

	if !containsLog(*res, "[DRY-RUN] would commit + push") {
		t.Errorf("missing DRY-RUN log; got: %v", res.Logs)
	}

	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun should not commit; HEAD moved from %s to %s", headBefore, headAfter)
	}
}

func TestShipDirect_DryRun_NoStagedChanges_CleanExit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	res := &RunResult{}
	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "dry: nothing staged",
		ProjectRoot:   repo,
		DryRun:        true,
		Runner:        execRunner,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	if err := shipDirect(context.Background(), opts, res, "main"); err != nil {
		t.Fatalf("DryRun clean tree should not error: %v", err)
	}
	if !containsLog(*res, "no staged changes to ship") {
		t.Errorf("missing clean-exit log: %v", res.Logs)
	}
}

func TestShipFromWorktree_DryRun_LogsAndNoFFMerge(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "dry-run-branch")
	mustWrite(t, filepath.Join(wt, "dry-change.txt"), "dry content\n")
	runGit(t, wt, "add", "dry-change.txt")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":99,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	headBefore := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "dry: worktree dry run",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("DryRun worktree ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("DryRun should ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "[DRY-RUN]") {
		t.Errorf("missing DRY-RUN log lines: %v", res.Logs)
	}

	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun must not ff-merge; HEAD moved %s → %s", headBefore, headAfter)
	}
}

func TestRun_DryRun_WritesJournal(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "dry.txt"), "something\n")
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "dry: end-to-end",
		DryRun:        true,
	})
	if err != nil {
		t.Fatalf("DryRun Run errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("DryRun Run want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if res.DryRunPath == "" {
		t.Fatal("DryRunPath must be set after a successful dry run")
	}
	if _, err := os.Stat(res.DryRunPath); err != nil {
		t.Fatalf("DryRunPath %s not on disk: %v", res.DryRunPath, err)
	}
}

func TestRun_DryRun_ClassManual_WritesJournal_NoCommit(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "manual.txt"), "manual dry change\n")

	headBefore := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	res, err := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "manual: dry run",
		DryRun:        true,
		AutoConfirm:   "test",
	})
	if err != nil {
		t.Fatalf("DryRun manual errored: %v", err)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("DryRun manual want ExitOK, got %d", res.ExitCode)
	}
	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun must not commit; HEAD moved %s → %s", headBefore, headAfter)
	}
}

func TestShipFromWorktree_CleanWorktreeAheadBranch_Merges(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "ahead-branch")

	mustWrite(t, filepath.Join(wt, "already-committed.txt"), "already in wt\n")
	runGit(t, wt, "add", "-A")
	runGit(t, wt, "-c", "commit.gpgsign=false", "commit", "-m", "pre-committed in worktree")

	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":7,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: ahead branch ship"})
	if err != nil {
		t.Fatalf("ahead-branch ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "ff-merged ahead-branch into main") {
		t.Errorf("missing ff-merge log; got %v", res.Logs)
	}
	mainLog := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainLog, "already-committed.txt") {
		t.Errorf("ahead-branch commit not on main; HEAD log files: %q", mainLog)
	}
}
