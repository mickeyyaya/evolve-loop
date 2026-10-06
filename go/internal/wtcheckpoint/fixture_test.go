package wtcheckpoint_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/wtcheckpoint"
)

type hubFixture struct {
	t     *testing.T
	store *gittest.Repo
	root  string
}

func newHub(t *testing.T) hubFixture {
	t.Helper()
	src := gittest.Fixture(t)
	writeFile(t, src.Dir, "a.txt", "a\n")
	writeFile(t, src.Dir, "b.txt", "b\n")
	writeFile(t, src.Dir, "go/evolve", "binary-v1\n")
	writeFile(t, src.Dir, ".gitignore", "*.log\nout/\n")
	src.Git("add", "-A")
	src.Git("commit", "-q", "-m", "base")
	store := gittest.Bare(t)
	src.Git("push", "-q", store.Dir, "main")
	store.Git("update-ref", "refs/remotes/origin/main", "main")
	return hubFixture{t: t, store: store, root: filepath.Dir(store.Dir)}
}

func (h hubFixture) addWorktree(rel, branch string) wtcheckpoint.Worktree {
	h.t.Helper()
	dir := filepath.Join(h.root, rel)
	h.store.Git("worktree", "add", "-q", "-b", branch, dir, "main")
	return wtcheckpoint.Worktree{Dir: dir, Name: filepath.Base(dir)}
}

func (h hubFixture) hub() wtcheckpoint.Hub {
	h.t.Helper()
	hub, err := wtcheckpoint.ResolveHub(h.addWorktreeOnce())
	if err != nil {
		h.t.Fatal(err)
	}
	return hub
}

func (h hubFixture) addWorktreeOnce() string {
	h.t.Helper()
	dir := filepath.Join(h.root, "console")
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	h.store.Git("worktree", "add", "-q", "--detach", dir, "main")
	return dir
}

func (h hubFixture) git(dir string, args ...string) string {
	h.t.Helper()
	out, err := gitexec.Default(dir).Output(context.Background(), args...)
	if err != nil {
		h.t.Fatalf("git %s in %s: %v", strings.Join(args, " "), dir, err)
	}
	return out
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dirtyThreeWays(t *testing.T, h hubFixture, dir string) {
	t.Helper()
	writeFile(t, dir, "staged.txt", "staged\n")
	h.git(dir, "add", "staged.txt")
	writeFile(t, dir, "a.txt", "a edited, unstaged\n")
	writeFile(t, dir, "notes/untracked.txt", "untracked\n")
}

func clockAt(stamps ...string) func() time.Time {
	i := 0
	return func() time.Time {
		at, err := time.Parse("20060102T150405Z", stamps[i])
		if err != nil {
			panic(err)
		}
		if i < len(stamps)-1 {
			i++
		}
		return at
	}
}

func lines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(strings.TrimSpace(s), "\n")
}

func gitIn(dir string, args ...string) (string, string, int, error) {
	return gitexec.Default(dir).Capture(context.Background(), args...)
}

func uniqueClock(i int) func() time.Time {
	at := time.Date(2026, 10, 6, 6, 0, i, 0, time.UTC)
	return func() time.Time { return at }
}
