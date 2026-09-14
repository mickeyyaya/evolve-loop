//go:build integration

package core

import (
	"path/filepath"
	"testing"
)

// detachedWorktree provisions what production provisions: a detached git
// worktree of repo at HEAD, in its own temp dir. Since 2026-09-14 a cycle
// never mutates its project root (inPlaceWorktree), so a fixture that hands
// the repository to the orchestrator as its own worktree exercises the
// refusal, not the mutating path it was written to pin.
func detachedWorktree(t *testing.T, repo string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "worktree")
	gitInRepo(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	return wt
}
