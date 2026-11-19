package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gcFlagKeyBranch = "cycle-flagkey-1"

func gcFlagKeyRun(t *testing.T, root string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("evolve gc rc=%d stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func gcFlagKeyInboxItems(t *testing.T, root string) []string {
	t.Helper()
	items, err := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "*gc-flag-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestGC_ABranchFlaggedWithAndThenWithoutItsWorktreeMintsOneInboxItem(t *testing.T) {
	root, _, _ := gcWorktreeEnv(t, "")
	worktree := filepath.Join(root, ".evolve", "worktrees", gcFlagKeyBranch)
	gcGit(t, root, "worktree", "add", "-q", "-b", gcFlagKeyBranch, worktree, "main")
	if err := os.WriteFile(filepath.Join(worktree, "unlanded.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gcGit(t, worktree, "add", "unlanded.txt")
	gcGit(t, worktree, "commit", "-q", "-m", "unlanded work")

	for run := 1; run <= 2; run++ {
		if out := gcFlagKeyRun(t, root); !strings.Contains(out, gcFlagKeyBranch) {
			t.Fatalf("run %d with the worktree did not flag %s:\n%s", run, gcFlagKeyBranch, out)
		}
	}
	gcGit(t, root, "worktree", "remove", "--force", worktree)
	for run := 3; run <= 4; run++ {
		if out := gcFlagKeyRun(t, root); !strings.Contains(out, gcFlagKeyBranch) {
			t.Fatalf("run %d without the worktree did not flag %s:\n%s", run, gcFlagKeyBranch, out)
		}
	}

	if items := gcFlagKeyInboxItems(t, root); len(items) != 1 {
		t.Errorf("one branch flagged on four runs minted %d inbox items %v, want exactly 1", len(items), items)
	}
}
