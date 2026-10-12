package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestRunWorktreeCreateAndCleanup_RefuseAnUnknownFlag(t *testing.T) {
	t.Parallel()
	for _, sub := range []string{"create", "cleanup"} {
		var out, errb bytes.Buffer
		if code := runWorktree([]string{sub, "--nope"}, nil, &out, &errb); code != 10 || out.Len() != 0 || !strings.Contains(errb.String(), "flag provided but not defined: -nope") {
			t.Errorf("%s: code %d stdout %q stderr %q", sub, code, out.String(), errb.String())
		}
	}
}

func TestRunWorktreeCreate_DefaultsTheBaseUnderTheProjectEvolveDir(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "base")
	root, err := filepath.EvalSymlinks(repo.Dir)
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runWorktree([]string{"create", "--cycle", "4", "--lane", "e3lane", "--project-root", root}, nil, &out, &errb)

	if code != 0 {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
	wt := strings.TrimSpace(out.String())
	if filepath.Dir(wt) != filepath.Join(root, ".evolve", "worktrees") || !strings.Contains(filepath.Base(wt), "4") {
		t.Errorf("worktree %q is not a cycle-4 dir under %s", wt, filepath.Join(root, ".evolve", "worktrees"))
	}
	if _, err := os.Stat(filepath.Join(wt, ".git")); err != nil {
		t.Errorf("no checkout at %s: %v", wt, err)
	}
}

func TestRunWorktreeCreate_AnUncreatableBaseExitsOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer

	code := runWorktree([]string{"create", "--cycle", "4", "--project-root", root, "--base", filepath.Join(blocker, "wts")}, nil, &out, &errb)

	if code != 1 || out.Len() != 0 || !strings.HasPrefix(errb.String(), "evolve worktree create: mkdir: ") {
		t.Fatalf("code %d stdout %q stderr %q", code, out.String(), errb.String())
	}
}
