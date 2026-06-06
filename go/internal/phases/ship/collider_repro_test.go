// collider_repro_test.go — bug reproducer for the SIXTH drift mode:
// phases with writes_source=false (e.g. bug-reproduction) write proposal
// files into the main repo tree during cycle execution. When the cycle
// worktree commits those same paths, git merge --ff-only refuses with
// "would overwrite untracked file" — but the current code maps this to
// CodeGitFFMergeDiverged ("divergent history") without naming the collider.
//
// FAILS on the current tree: no checkUntrackedColliders pre-flight exists.
// PASSES once the fix adds checkUntrackedColliders in gitops.go and defines
// CodeGitUntrackedCollider in shiperror.go.
package ship

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBugRepro_ColliderFFMerge demonstrates the SIXTH drift mode bug:
// ship must fail with GIT_UNTRACKED_COLLIDER (naming the file) rather than
// GIT_FF_MERGE_DIVERGED (generic "divergent history") when the main tree has
// untracked files that collide with the cycle-branch commit tree.
//
// Root cause: writes_source=false phases (bug-reproduction, test-amplification,
// tdd-predicates) write proposal artifacts into the main repo tree.
// When the cycle worktree commits those same paths, ff-merge refuses.
// The ship command currently loops to recovery-audit 3× without ever shipping
// and never tells the operator which file caused the failure.
func TestBugRepro_ColliderFFMerge(t *testing.T) {
	// colliderPath is a file that a writes_source=false phase would drop into
	// the main repo tree as a "proposal" artifact.
	const colliderPath = "acs/cycle-231/bug-reproduction-report.md"

	// --- Setup: main repo + bare remote ---
	repo := makeRepo(t)
	addRemote(t, repo)

	// --- Setup: cycle worktree with a commit that includes the collider file ---
	wt := makeWorktree(t, repo, "cycle-1-branch")

	mustMkdir(t, filepath.Join(wt, filepath.Dir(colliderPath)))
	mustWrite(t, filepath.Join(wt, colliderPath),
		"# Bug Reproduction Report\n<!-- cycle-231 proposal -->\n")
	runGit(t, wt, "add", colliderPath)
	runGit(t, wt, "-c", "commit.gpgsign=false",
		"commit", "-m", "test: add bug-reproduction report to cycle branch")

	// --- Inject the bug: create the SAME file as UNTRACKED in the main tree ---
	// This mimics a writes_source=false phase that wrote its output into the
	// repo tree instead of the run workspace.
	mustMkdir(t, filepath.Join(repo, filepath.Dir(colliderPath)))
	mustWrite(t, filepath.Join(repo, colliderPath),
		"# stale proposal — should live in .evolve/runs/cycle-231/\n")

	// Sanity: the file must be untracked in main (not staged, not committed).
	untracked := runGitOut(t, repo, "ls-files", "--others", "--exclude-standard")
	if !strings.Contains(untracked, filepath.ToSlash(colliderPath)) {
		t.Fatalf("test setup error: collider %q is not untracked in main tree\nls-files output: %s", colliderPath, untracked)
	}

	// --- Setup: cycle-state.json pointing at the worktree ---
	mustWrite(t, filepath.Join(repo, ".evolve", "cycle-state.json"),
		`{"cycle_id":1,"phase":"ship","active_worktree":"`+wt+`"}`)

	// --- Setup: PASS audit (untracked files don't affect git diff HEAD tree SHA) ---
	seedAudit(t, repo, "PASS")

	// --- Exercise: run ship --class cycle ---
	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "test: cycle 1 with collider",
	})

	// --- Assert: ship must fail, not silently succeed or loop ---
	if res.ExitCode == ExitOK {
		t.Fatal("ship succeeded despite untracked collider in main tree; expected failure with GIT_UNTRACKED_COLLIDER")
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	// The error must use GIT_UNTRACKED_COLLIDER (pre-flight), not
	// GIT_FF_MERGE_DIVERGED (post-attempt with generic "divergent history").
	// On the current tree: this assertion fails — code is GIT_FF_MERGE_DIVERGED.
	// After fix: checkUntrackedColliders returns GIT_UNTRACKED_COLLIDER.
	if !strings.Contains(errMsg, "GIT_UNTRACKED_COLLIDER") {
		t.Errorf("expected error code GIT_UNTRACKED_COLLIDER, got: %s", errMsg)
	}

	// The error must name the specific collider file so operators know what to delete.
	// On the current tree: error says "divergent history" with no filename.
	// After fix: checkUntrackedColliders includes the file path in the message.
	if !strings.Contains(errMsg, colliderPath) {
		t.Errorf("expected error to name collider file %q, got: %s", colliderPath, errMsg)
	}

	// The go/evolve binary should NOT have been dirty-reset during a pre-flight
	// failure (the reset only makes sense just before an ff-merge attempt).
	// Check: main HEAD has not moved (no accidental commit or merge happened).
	mainHead := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "HEAD"))
	cycleHead := strings.TrimSpace(runGitOut(t, repo, "rev-parse", "cycle-1-branch"))
	if mainHead == cycleHead {
		t.Errorf("main HEAD advanced to cycle branch HEAD despite pre-flight failure; ff-merge must not have been attempted")
	}

	// The collider file must still be untracked — pre-flight must not delete it.
	if _, err := os.Stat(filepath.Join(repo, colliderPath)); err != nil {
		t.Errorf("pre-flight must not delete the collider file: %v", err)
	}
	untrackedAfter := runGitOut(t, repo, "ls-files", "--others", "--exclude-standard")
	if !strings.Contains(untrackedAfter, filepath.ToSlash(colliderPath)) {
		t.Errorf("pre-flight must not remove the collider file; it should remain untracked for operator triage")
	}
}
