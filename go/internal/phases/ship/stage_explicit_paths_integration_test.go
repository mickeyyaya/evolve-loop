//go:build integration

// stage_explicit_paths_integration_test.go — real-git half of the cycle-1067
// `ship-stage-explicit-paths` contract. The unit half (stage_explicit_paths_test.go)
// pins the git ARGUMENTS via a capture runner; this pins the OBSERVABLE EFFECT
// against a genuine repository: the ship commit contains the declared path and
// does NOT contain an undeclared untracked stray that `git add -A` would sweep
// in (the cross-lane leak of cycle-645).
package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShipFromWorktree_StagesDeclaredPathsOnly_ExcludesUndeclaredStray —
// RED today: shipFromWorktree stages with `git add -A` (gitops.go:374), so the
// stray rides into the cycle commit.
func TestShipFromWorktree_StagesDeclaredPathsOnly_ExcludesUndeclaredStray(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)

	// Un-stage so the ship's own staging step is what decides the commit.
	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	// An undeclared untracked file in the worktree — a sibling lane's leak.
	mustWrite(t, filepath.Join(wt, "sibling-lane-leak.txt"), "not declared by any phase report\n")

	// Workspace whose phase reports declare ONLY wt-change.txt.
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"),
		"# Build Report\n\n## Files Changed\n\n- `wt-change.txt`\n")
	mustWrite(t, filepath.Join(ws, "test-report.md"), "# TDD Report\n\nno additional paths\n")

	opts := &Options{
		Class:         ClassCycle,
		CommitMessage: "feat: explicit worktree staging",
		ProjectRoot:   repo,
		PluginRoot:    repo,
		WorkspacePath: ws,
		Stdout:        io.Discard,
		Stderr:        io.Discard,
	}
	if err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree: %v", err)
	}

	files := commitFileList(t, wt, "cycle-1")
	if !strings.Contains(files, "wt-change.txt") {
		t.Errorf("declared path wt-change.txt absent from the ship commit; files=%q", files)
	}
	if strings.Contains(files, "sibling-lane-leak.txt") {
		t.Errorf("RED (add -A sweep): undeclared stray rode into the ship commit; files=%q", files)
	}
	// The stray must still exist on disk — excluded from staging, not deleted.
	if _, err := os.Stat(filepath.Join(wt, "sibling-lane-leak.txt")); err != nil {
		t.Errorf("explicit staging must not remove the undeclared file: %v", err)
	}
}

// commitFileList returns the newline-joined file list of the tip commit of
// branch in the given git dir.
func commitFileList(t *testing.T, dir, branch string) string {
	t.Helper()
	return runGitOut(t, dir, "show", "--pretty=format:", "--name-only", branch)
}
