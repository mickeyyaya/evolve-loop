package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func devHubFixture(t *testing.T) (runtime string, seed *gittest.Repo) {
	t.Helper()
	origin := gittest.Bare(t)
	seed = gittest.Fixture(t)
	seed.Git("commit", "-q", "--allow-empty", "-m", "base")
	seed.Git("remote", "add", "origin", origin.Dir)
	seed.Git("push", "-q", "origin", "main")
	hub := t.TempDir()
	store := filepath.Join(hub, ".repo.git")
	seed.Git("clone", "-q", "--bare", origin.Dir, store)
	for _, kv := range append(gittest.MaintenanceConfig(), [2]string{"user.name", "u"}, [2]string{"user.email", "u@example.com"}) {
		brGit(t, store, "config", kv[0], kv[1])
	}
	runtime = filepath.Join(hub, "runtime")
	brGit(t, store, "worktree", "add", "-q", runtime, "main")
	return runtime, seed
}

func runDev(t *testing.T, args ...string) (string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runWorktree(args, nil, &out, &errb)
	return out.String() + errb.String(), code
}

func TestWorktreeDev_CreateAtOriginTipThenCleanupAfterMerge(t *testing.T) {
	t.Parallel()
	runtime, seed := devHubFixture(t)
	seed.Git("commit", "-q", "--allow-empty", "-m", "origin moves on")
	seed.Git("push", "-q", "origin", "main")
	out, code := runDev(t, "create", "--dev", "t1", "--branch", "b1", "--project-root", runtime)
	if code != 0 {
		t.Fatalf("create --dev exit=%d\n%s", code, out)
	}
	dev := filepath.Join(filepath.Dir(runtime), "dev", "t1")
	if got := strings.TrimSpace(out); resolvedPath(got) != resolvedPath(dev) {
		t.Errorf("create --dev printed %q, want %q", got, dev)
	}
	if head := seed.Git("rev-parse", "main"); brOut(t, dev, "rev-parse", "HEAD") != head {
		t.Errorf("dev tree must start at the fetched origin tip %s", head)
	}
	if out, code := runDev(t, "create", "--dev", "t1", "--branch", "b2", "--project-root", runtime); code != 1 {
		t.Errorf("second create of t1 exit=%d, want 1\n%s", code, out)
	}
	if err := os.WriteFile(filepath.Join(dev, "f.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := runDev(t, "cleanup", "--dev", "t1", "--project-root", runtime); code != 1 || !strings.Contains(out, "dirty") {
		t.Errorf("cleanup of a dirty tree exit=%d, want 1 naming dirty\n%s", code, out)
	}
	brGit(t, dev, "add", "f.txt")
	brGit(t, dev, "commit", "-q", "-m", "work")
	olderPRHead := func(context.Context, string, string) (string, error) { return strings.Repeat("b", 40) + "\n", nil }
	var refusedOut, refusedErr bytes.Buffer
	if code := runWorktreeCleanupDev(runtime, "t1", olderPRHead, &refusedOut, &refusedErr); code != 1 || !strings.Contains(refusedErr.String(), "not merged") {
		t.Errorf("cleanup of an unmerged branch exit=%d, want 1 naming not merged\n%s%s", code, refusedOut.String(), refusedErr.String())
	}
	brGit(t, dev, "push", "-q", "origin", "HEAD:main")
	if out, code := runDev(t, "cleanup", "--dev", "t1", "--project-root", runtime); code != 0 {
		t.Fatalf("cleanup of a merged tree exit=%d\n%s", code, out)
	}
	if _, err := os.Stat(dev); !os.IsNotExist(err) {
		t.Errorf("cleanup left %s behind", dev)
	}
	if brBranchExists(t, filepath.Join(filepath.Dir(runtime), ".repo.git"), "b1") {
		t.Error("cleanup left branch b1 behind")
	}
}

func TestWorktreeDev_PlainCheckoutIsNotAHub(t *testing.T) {
	t.Parallel()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	out, code := runDev(t, "create", "--dev", "t1", "--branch", "b1", "--project-root", r.Dir)
	if code != 1 || !strings.Contains(out, "not a hub") {
		t.Errorf("create --dev outside a hub exit=%d, want 1 naming the hub layout\n%s", code, out)
	}
	if brBranchExists(t, r.Dir, "b1") {
		t.Error("a refused create left branch b1 behind")
	}
}

func TestMergedPRHeadIs_OnlyAnExactHeadMatchProvesTheMerge(t *testing.T) {
	t.Parallel()
	head := strings.Repeat("a", 40)
	cases := []struct {
		name, oids string
		want       bool
	}{
		{"no merged PR", "", false},
		{"merged PR at the local head", head + "\n", true},
		{"local head is in a later of several merged PRs", strings.Repeat("b", 40) + "\n" + head + "\n", true},
		{"merged PR at an older head, local commit after it", strings.Repeat("b", 40) + "\n", false},
		{"prefix of the head is not the head", head[:12] + "\n", false},
	}
	for _, c := range cases {
		if got := mergedPRHeadIs(c.oids, head); got != c.want {
			t.Errorf("%s: mergedPRHeadIs(%q) = %v, want %v", c.name, c.oids, got, c.want)
		}
	}
}

func brOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitexec.Default(dir).Output(context.Background(), args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
