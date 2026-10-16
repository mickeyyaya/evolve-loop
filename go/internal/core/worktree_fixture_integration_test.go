//go:build integration

package core

import (
	"path/filepath"
	"testing"
)

func detachedWorktree(t *testing.T, repo string) string {
	t.Helper()
	wt := filepath.Join(t.TempDir(), "worktree")
	gitInRepo(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	return wt
}
