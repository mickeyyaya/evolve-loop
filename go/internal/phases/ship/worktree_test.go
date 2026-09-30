//go:build integration

package ship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShipFromWorktree_HappyPath_FFMergesAndWritesBinding(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	// PASS audit bound to main's clean HEAD/tree (worktree edits don't touch
	// main's working tree, so the diff-based binding still matches).
	seedAudit(t, repo, "PASS")

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: worktree ship"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (err=%v, logs=%v)", res.ExitCode, err, res.Logs)
	}
	if !containsLog(res, "worktree-aware ship") {
		t.Errorf("missing worktree-aware log: %v", res.Logs)
	}
	if !containsLog(res, "ff-merged cycle-1-branch into main") {
		t.Errorf("missing ff-merge log: %v", res.Logs)
	}
	if res.CommitSHA == "" {
		t.Error("expected non-empty CommitSHA")
	}
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("worktree edit not merged into main; HEAD files: %q", mainFiles)
	}
	raw, rerr := os.ReadFile(filepath.Join(repo, ".evolve", "runs", "cycle-1", "ship-binding.json"))
	if rerr != nil {
		t.Fatalf("ship-binding.json not written: %v", rerr)
	}
	var binding map[string]any
	if jerr := json.Unmarshal(raw, &binding); jerr != nil {
		t.Fatalf("ship-binding.json invalid JSON: %v", jerr)
	}
	if binding["commit_sha"] != res.CommitSHA {
		t.Errorf("binding commit_sha=%v, want %s", binding["commit_sha"], res.CommitSHA)
	}
	if fmt.Sprintf("%v", binding["cycle"]) != "1" {
		t.Errorf("binding cycle=%v, want 1", binding["cycle"])
	}
}

func TestShipFromWorktree_TreeSHAMismatch_VerifiesBeforeCommit(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-9-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":9,"phase":"ship","active_worktree":"`+wt+`"}`)
	// Bogus audit-bound tree SHA — will never equal the real staged tree.
	seedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("a", 40))

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: should breach pre-commit"})

	if res.ExitCode != ExitFailure {
		t.Fatalf("want ExitFailure (predicate tree mismatch before commit), got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if containsLog(res, "committed in worktree") {
		t.Errorf("commit was created before tree-SHA verification — commit-then-fail window not closed: %v", res.Logs)
	}
	ahead := strings.TrimSpace(runGitOut(t, repo, "rev-list", "--count", "main..cycle-9-branch"))
	if ahead != "0" {
		t.Errorf("cycle branch advanced; main..branch ahead=%s", ahead)
	}
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("main advanced despite breach; files: %q", mainFiles)
	}
}

func TestShipFromWorktree_PreCommitBindingMatch_CommitsAndShips(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-10-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	runGit(t, wt, "add", "feature.txt")
	// The tree a commit from this staged index would carry == the bound tree.
	stagedTree := strings.TrimSpace(runGitOut(t, wt, "write-tree"))
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":10,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAuditWithBoundTree(t, repo, "PASS", stagedTree)

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: bound match ship"})
	if err != nil {
		t.Fatalf("bound-match ship errored: %v (logs=%v)", err, res.Logs)
	}
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "pre-commit tree-SHA binding verified") {
		t.Errorf("missing pre-commit verified log: %v", res.Logs)
	}
	if !containsLog(res, "committed in worktree") {
		t.Errorf("expected a commit after verification passed: %v", res.Logs)
	}
	// The verified worktree edit must now live on main.
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("worktree edit not merged into main; HEAD files: %q", mainFiles)
	}
}

func TestShipFromWorktree_CleanWorktreeNotAhead_ExitsCleanly(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-3-branch") // at main, no edits
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":3,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: nothing to ship"})
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "no changes in worktree AND branch not ahead") {
		t.Errorf("missing clean-exit log: %v", res.Logs)
	}
}

func TestShipFromWorktree_AcquiresAndReleasesShipLock(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	var acquired, released int
	var lockedPath string
	opts := Options{
		Class:         ClassCycle,
		CommitMessage: "feat: worktree ship",
		shipLock: func(path string) (func(), error) {
			acquired++
			lockedPath = path
			return func() { released++ }, nil
		},
	}
	res, err := runShip(t, repo, opts)
	if res.ExitCode != ExitOK {
		t.Fatalf("want ExitOK, got %d (err=%v logs=%v)", res.ExitCode, err, res.Logs)
	}
	if acquired != 1 || released != 1 {
		t.Fatalf("ship lock acquired=%d released=%d, want 1/1", acquired, released)
	}
	if filepath.Base(lockedPath) != "ship.lock" {
		t.Errorf("locked %q, want a path ending in ship.lock", lockedPath)
	}
}

func TestShipFromWorktree_DryRun_SkipsShipLock(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-1-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)
	seedAudit(t, repo, "PASS")

	var acquired int
	opts := Options{
		Class:         ClassCycle,
		DryRun:        true,
		CommitMessage: "feat: worktree ship",
		shipLock:      func(string) (func(), error) { acquired++; return func() {}, nil },
	}
	if _, err := runShip(t, repo, opts); err != nil {
		t.Fatalf("dry-run ship: %v", err)
	}
	if acquired != 0 {
		t.Errorf("dry-run must not acquire the ship lock; acquired=%d", acquired)
	}
}
