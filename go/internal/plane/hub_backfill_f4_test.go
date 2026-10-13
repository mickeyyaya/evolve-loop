package plane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHub_ALinkedWorktreeWhoseGitDirHasNoCommondirIsRefusedWithTheCause(t *testing.T) {
	root := t.TempDir()
	gitdir := filepath.Join(t.TempDir(), "worktrees", "wt")
	if err := os.MkdirAll(gitdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hub, err := ResolveHub(root)

	if err == nil || !strings.Contains(err.Error(), "linked worktree without commondir") || hub != (Hub{}) {
		t.Fatalf("hub=%+v err=%v", hub, err)
	}
}
