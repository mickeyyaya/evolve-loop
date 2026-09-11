//go:build integration

package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewCycleRun_RejectsRootCycleOccupiedInFetchedWorktree(t *testing.T) {
	projectRoot := staleRootCycleSourceCheckout(t)
	storage := &fakeUpdaterStorage{}
	orchestrator := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil))

	_, cleanup, err := orchestrator.newCycleRun(context.Background(), CycleRequest{
		ProjectRoot:           projectRoot,
		GoalHash:              "goal",
		DisableWorkspaceGuard: true,
	})
	if cleanup != nil {
		cleanup(false, true)
	}
	if err == nil || !strings.Contains(err.Error(), "cycle42") || !strings.Contains(err.Error(), "provisioned source") {
		t.Fatalf("newCycleRun error = %v, want fetched root-module cycle42 collision", err)
	}
	if storage.mem.st.LastAllocatedCycleNumber != 42 {
		t.Fatalf("collision lease = %d, want burned cycle 42", storage.mem.st.LastAllocatedCycleNumber)
	}
	if storage.writeCSCalls != 0 {
		t.Fatalf("cycle state persisted %d times before source collision was rejected", storage.writeCSCalls)
	}
	worktrees, globErr := filepath.Glob(filepath.Join(projectRoot, ".evolve", "worktrees", "cycle-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(worktrees) != 1 {
		t.Fatalf("colliding fetched worktree count = %d, want one preserved path: %v", len(worktrees), worktrees)
	}
	if !strings.Contains(err.Error(), worktrees[0]) {
		t.Fatalf("collision error does not report preserved worktree %s: %v", worktrees[0], err)
	}
	t.Cleanup(func() { _ = (gitWorktree{}).Cleanup(projectRoot, worktrees[0]) })
}

type cycleSourceGitFixture struct {
	t *testing.T
}

func newCycleSourceGitFixture(t *testing.T) cycleSourceGitFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	hooks := t.TempDir()
	for key, value := range map[string]string{
		"GIT_AUTHOR_EMAIL":    "ci@example.com",
		"GIT_AUTHOR_NAME":     "ci",
		"GIT_COMMITTER_EMAIL": "ci@example.com",
		"GIT_COMMITTER_NAME":  "ci",
		"GIT_CONFIG_COUNT":    "2",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_KEY_0":    "commit.gpgsign",
		"GIT_CONFIG_KEY_1":    "core.hooksPath",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_VALUE_0":  "false",
		"GIT_CONFIG_VALUE_1":  hooks,
		"GIT_TERMINAL_PROMPT": "0",
	} {
		t.Setenv(key, value)
	}
	return cycleSourceGitFixture{t: t}
}

func (g cycleSourceGitFixture) run(dir string, args ...string) {
	g.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		g.t.Fatalf("git %v in %s timed out: %s", args, dir, out)
	}
	if err != nil {
		g.t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func staleRootCycleSourceCheckout(t *testing.T) string {
	t.Helper()
	git := newCycleSourceGitFixture(t)
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	git.run(root, "init", "-q", "--bare", "-b", "main", origin)

	seed := filepath.Join(root, "seed")
	if err := os.Mkdir(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	git.run(seed, "init", "-q", "-b", "main")
	writeRootCycleSourceCommit(t, git, seed, 41, "initial source")
	git.run(seed, "remote", "add", "origin", origin)
	git.run(seed, "push", "-q", "-u", "origin", "main")

	checkout := filepath.Join(root, "checkout")
	git.run(root, "clone", "-q", origin, checkout)
	writeRootCycleSourceCommit(t, git, seed, 42, "newer source")
	git.run(seed, "push", "-q", "origin", "main")
	return checkout
}

func writeRootCycleSourceCommit(t *testing.T, git cycleSourceGitFixture, repo string, cycle int, message string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, "acs", "cycle"+strconv.Itoa(cycle)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/cycle-source\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	predicate := filepath.Join(repo, "acs", "cycle"+strconv.Itoa(cycle), "predicate_test.go")
	if err := os.WriteFile(predicate, []byte("package cycle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git.run(repo, "add", "go.mod", "acs")
	git.run(repo, "commit", "-q", "-m", message)
}
