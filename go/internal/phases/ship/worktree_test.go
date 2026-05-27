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

// makeWorktree adds a git worktree of repo on a fresh branch at main and
// returns its absolute path. The worktree shares repo's object store, so a
// commit there is ff-mergeable into main.
func makeWorktree(t *testing.T, repo, branch string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", "-b", branch, wt, "main")
	return wt
}

// seedAuditWithBoundTree is seedAudit plus an `audit_bound_tree_sha:` line
// in the report body, so verifyAuditBinding stashes it into
// opts.internalAuditBoundTreeSHA and gitops enforces the pre-merge check.
func seedAuditWithBoundTree(t *testing.T, repo, verdict, boundTreeSHA string) {
	t.Helper()
	auditPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "audit-report.md")
	body := fmt.Sprintf("<!-- challenge-token: testtoken123 -->\n# Audit Report — Cycle 1\n\nVerdict: %s\naudit_bound_tree_sha: %s\n\nAll criteria met (test fixture).\n", verdict, boundTreeSHA)
	mustWrite(t, auditPath, body)
	sha := mustHashFile(t, auditPath)
	headSHA := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	treeSHA := treeStateSHA(t, repo)
	entry := map[string]any{
		"ts": "2026-04-27T00:00:00Z", "cycle": 1, "role": "auditor",
		"kind": "agent_subprocess", "model": "sonnet", "exit_code": 0,
		"duration_s": "30", "artifact_path": auditPath, "artifact_sha256": sha,
		"challenge_token": "testtoken123", "git_head": headSHA, "tree_state_sha": treeSHA,
	}
	line, _ := json.Marshal(entry)
	mustWrite(t, filepath.Join(repo, ".evolve", "ledger.jsonl"), string(line)+"\n")
}

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

// TestShipFromWorktree_PreMergeTreeSHAMismatch_BreachAndRollback: the
// v10.15.0 pre-merge binding refuses to ff-merge when the audit-bound tree
// SHA != the worktree commit's tree, and rolls the worktree commit back.
func TestShipFromWorktree_PreMergeTreeSHAMismatch_BreachAndRollback(t *testing.T) {
	repo := makeRepo(t)
	addRemote(t, repo)
	wt := makeWorktree(t, repo, "cycle-2-branch")
	mustWrite(t, filepath.Join(wt, "feature.txt"), "worktree feature\n")
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":2,"phase":"ship","active_worktree":"`+wt+`"}`)
	// Bogus audit-bound tree SHA — will never equal the real worktree tree.
	seedAuditWithBoundTree(t, repo, "PASS", strings.Repeat("a", 40))

	res, _ := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: should breach"})
	if res.ExitCode != ExitIntegrity {
		t.Fatalf("want ExitIntegrity, got %d (logs=%v)", res.ExitCode, res.Logs)
	}
	if !containsLog(res, "INTEGRITY BREACH (pre-merge)") {
		t.Errorf("missing pre-merge breach log: %v", res.Logs)
	}
	// main must NOT have advanced (the ff-merge never ran).
	mainFiles := runGitOut(t, repo, "log", "-1", "--name-only", "--format=")
	if strings.Contains(mainFiles, "feature.txt") {
		t.Errorf("main advanced despite breach; files: %q", mainFiles)
	}
	// Worktree commit was rolled back: cycle branch no longer ahead of main.
	ahead := strings.TrimSpace(runGitOut(t, repo, "rev-list", "--count", "main..cycle-2-branch"))
	if ahead != "0" {
		t.Errorf("worktree commit not rolled back; main..branch ahead=%s", ahead)
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
