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

func TestShipFromWorktree_StagesDeclaredPathsOnly_ExcludesUndeclaredStray(t *testing.T) {
	t.Parallel()
	repo, wt := makeWorktreeScenario(t)

	runGit(t, wt, "reset", "HEAD", "wt-change.txt")

	mustWrite(t, filepath.Join(wt, "sibling-lane-leak.txt"), "not declared by any phase report\n")

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
	if _, err := os.Stat(filepath.Join(wt, "sibling-lane-leak.txt")); err != nil {
		t.Errorf("explicit staging must not remove the undeclared file: %v", err)
	}
}

func commitFileList(t *testing.T, dir, branch string) string {
	t.Helper()
	return runGitOut(t, dir, "show", "--pretty=format:", "--name-only", branch)
}
