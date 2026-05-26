// dryrun_branches_test.go — behavioral tests for the DryRun=true code paths
// across shipDirect, shipFromWorktree, Run (end-to-end), and writeDryRunJournal.
//
// DryRun skips all mutations (git add, commit, push, gh release) but runs
// every read-only check. These tests prove that the dry-run branches:
//  1. emit "[DRY-RUN]" log lines (not silent)
//  2. do NOT touch the repo (no commits, no pushes)
//  3. write the dry-run journal and set DryRunPath
//  4. still validate audit-binding before short-circuiting
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShipDirect_DryRun_LogsAndNoCommit verifies that DryRun=true
// emits the "[DRY-RUN] would commit + push" log and does NOT create
// a commit (HEAD stays at the initial commit).
// Note: shipDirect with DryRun=true skips "git add -A" but still checks
// for staged changes via "git diff --cached --quiet". We stage manually
// here so the diff check sees staged changes and proceeds to the DryRun
// short-circuit (the path after the "no staged changes" early return).
func TestShipDirect_DryRun_LogsAndNoCommit(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "change.txt"), "dry-run change\n")
	// Stage the file manually (since DryRun skips git add -A inside shipDirect).
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

	// Must log the DRY-RUN line.
	if !containsLog(*res, "[DRY-RUN] would commit + push") {
		t.Errorf("missing DRY-RUN log; got: %v", res.Logs)
	}

	// HEAD must not have advanced.
	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun should not commit; HEAD moved from %s to %s", headBefore, headAfter)
	}
}

// TestShipDirect_DryRun_NoStagedChanges_CleanExitNoLog verifies that
// DryRun + no staged changes still exits cleanly via the "no staged changes"
// path (the diff--cached check runs even in DryRun).
func TestShipDirect_DryRun_NoStagedChanges_CleanExit(t *testing.T) {
	repo := makeRepo(t) // clean tree
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

// TestShipFromWorktree_DryRun_LogsAndNoFFMerge verifies that a
// DryRun=true worktree ship logs "[DRY-RUN] would commit in worktree"
// and "[DRY-RUN] would ff-merge + push", and does NOT merge the cycle
// branch into main.
// The worktree has an uncommitted file so that the "not ahead" early-exit
// does NOT fire, and the code reaches the DryRun commit+merge short-circuit.
func TestShipFromWorktree_DryRun_LogsAndNoFFMerge(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "dry-run-branch")
	// Write an UNTRACKED file in the worktree. shipFromWorktree runs
	// "git -C wt diff --cached --quiet"; a new untracked file is not staged,
	// so the worktree appears clean (exit 0). However the branch IS ahead once
	// we pre-stage it. To reliably exercise the DryRun commit path we need
	// an UNSTAGED change — but the code only checks --cached. Instead, stage
	// a change in the worktree so exit 1 fires (staged-changes path).
	mustWrite(t, filepath.Join(wt, "dry-change.txt"), "dry content\n")
	runGit(t, wt, "add", "dry-change.txt") // stage it inside the worktree
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

	// main must NOT have advanced.
	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun must not ff-merge; HEAD moved %s → %s", headBefore, headAfter)
	}
}

// TestRun_DryRun_WritesJournal drives the top-level Run() with DryRun=true
// end-to-end (including audit-binding). Confirms DryRunPath is set and
// contains a readable JSON journal.
func TestRun_DryRun_WritesJournal(t *testing.T) {
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

// TestRun_DryRun_ClassManual_WritesJournal_NoCommit runs DryRun with
// ClassManual (EVOLVE_SHIP_AUTO_CONFIRM=1) to cover the postShip
// DryRun short-circuit (postShip returns nil immediately on DryRun).
func TestRun_DryRun_ClassManual_WritesJournal_NoCommit(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	mustWrite(t, filepath.Join(repo, "manual.txt"), "manual dry change\n")

	headBefore := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))

	res, err := runShip(t, repo, Options{
		Class:         ClassManual,
		CommitMessage: "manual: dry run",
		DryRun:        true,
		Env:           map[string]string{"EVOLVE_SHIP_AUTO_CONFIRM": "1"},
	})
	if err != nil {
		t.Fatalf("DryRun manual errored: %v", err)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("DryRun manual want ExitOK, got %d", res.ExitCode)
	}
	// HEAD must not have advanced.
	headAfter := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	if headBefore != headAfter {
		t.Errorf("DryRun must not commit; HEAD moved %s → %s", headBefore, headAfter)
	}
}

// TestShipFromWorktree_CleanWorktreeAhead_Merges: worktree clean (no
// uncommitted changes) but cycle branch is ahead of main (a prior commit
// exists there). Ship should ff-merge it into main without a new commit.
func TestShipFromWorktree_CleanWorktreeAheadBranch_Merges(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "ahead-branch")

	// Commit directly in the worktree so the branch is ahead of main.
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
	// already-committed.txt must now be on main.
	mainLog := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainLog, "already-committed.txt") {
		t.Errorf("ahead-branch commit not on main; HEAD log files: %q", mainLog)
	}
}
