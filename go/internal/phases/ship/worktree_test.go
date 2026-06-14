//go:build integration

// worktree_test.go — coverage for the v8.43.0 worktree-aware ship path
// (shipFromWorktree + writeShipBinding). The 23-case native_test.go matrix
// ships directly from ProjectRoot and never sets cycle-state.json's
// active_worktree, so this entire path — commit-in-worktree, ff-merge into
// main, post-push tree-SHA binding, and the ship-binding.json sidecar —
// was previously 0% covered. These are the most irreversible operations in
// the package, so they earn dedicated behavioral tests.
package ship

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShipFromWorktree_HappyPath_FFMergesAndWritesBinding: Builder's edits
// live uncommitted in an active_worktree on a cycle branch. Ship must
// commit them there, ff-merge the cycle branch into main, push, and emit
// the ship-binding.json sidecar.
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
	// The worktree edit must now live on main.
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("worktree edit not merged into main; HEAD files: %q", mainFiles)
	}
	// ship-binding.json sidecar must exist and bind the committed SHA + cycle.
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

// TestShipFromWorktree_TreeSHAMismatch_VerifiesBeforeCommit: ADR-0048 Slice C1.
// The audit-bound tree-SHA binding is verified against the STAGED INDEX (via
// `git write-tree`) BEFORE the worktree commit, so a mismatch refuses with NO
// commit object ever created — eliminating the commit-then-rollback window.
// The distinguishing signal from the old post-commit-rollback behavior: the
// "committed in worktree" log MUST be absent (verification preceded mutation).
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

	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	// C1 invariant: verification ran BEFORE the commit — no commit was created.
	if containsLog(res, "committed in worktree") {
		t.Errorf("commit was created before tree-SHA verification — commit-then-fail window not closed: %v", res.Logs)
	}
	// The cycle branch must carry no new commit (never committed, not rolled back).
	ahead := strings.TrimSpace(runGitOut(t, repo, "rev-list", "--count", "main..cycle-9-branch"))
	if ahead != "0" {
		t.Errorf("cycle branch advanced; main..branch ahead=%s", ahead)
	}
	// main must be untouched.
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("main advanced despite breach; files: %q", mainFiles)
	}
}

// TestShipFromWorktree_PreCommitBindingMatch_CommitsAndShips: ADR-0048 Slice C1
// happy path — when the staged-index tree equals the audit-bound tree, the
// pre-commit verification passes, the commit is then made, and the cycle branch
// ff-merges into main. Proves verification precedes (and gates) the commit.
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

// TestShipFromWorktree_CleanWorktreeNotAhead_ExitsCleanly: an
// active_worktree with no uncommitted changes whose branch is not ahead of
// main is a no-op clean exit (audit was for an empty diff).
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

// TestShipFromWorktree_AcquiresAndReleasesShipLock pins ADR-0049 S5 / gap G1:
// the worktree-aware ship must hold the integrator lock across the shared-main
// critical section and release it. Inject a recording seam and assert it is
// acquired exactly once on <root>/.evolve/ship.lock and released. RED before
// the acquire is wired into shipFromWorktree (acquired=0), GREEN after.
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

// TestShipFromWorktree_DryRun_SkipsShipLock: a dry run mutates nothing, so it
// must NOT acquire the integrator lock (keeps dry-run pure + never creates the
// lock file).
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
