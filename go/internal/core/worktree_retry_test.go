package core

// worktree_retry_test.go — the cycles-1221/1231/1232/1234/1240 class: N lanes
// of one repo provision concurrently, `git worktree add` takes repo-level
// locks in the SHARED .git (the plane itself is a linked worktree), and a
// transient collision returns rc=255 with nothing but "Preparing worktree" on
// stderr. One collision killed the lane's whole cycle: ActiveWorktree stayed
// empty, CB.2 fail-fasted every dispatch exit=10, three identical fingerprints
// halted the batch — twice in one day, once with ZERO console git activity,
// proving lane-vs-lane contention. The alarm chain downstream is CORRECT; the
// defect is Create treating a transient lock failure as permanent.

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// The retry backoff is REAL time.Sleep in production. Without this init, every
// core test whose fixture makes worktree provisioning fail (the scenario
// engine does so by design, dozens of times) pays the full 2s+4s ladder —
// which ballooned the package from ~52s to >600s and timed out the commit-gate
// twice while looking exactly like host contention (the same misattribution
// the incident itself invited). Mirrors runner's settleSleep test-init no-op.
// Tests that COUNT sleeps install their own recorder and restore this no-op.
func init() { worktreeAddRetrySleep = func(time.Duration) {} }

// initRetryRepo makes a minimal real repo with one commit so `git worktree
// add` has a HEAD to cut from.
func initRetryRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		code, err := sysexec.DefaultRunner(context.Background(), "git", root, args, nil, nil, io.Discard, io.Discard)
		if err != nil || code != 0 {
			t.Fatalf("git %v: rc=%d err=%v", args, code, err)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "seed")
	return root
}

// failingAddRunner fails the first `worktree add` invocations with the live
// incident's exact shape (rc=255, "Preparing worktree" noise on stderr) and
// delegates everything else — and later attempts — to the real runner.
func failingAddRunner(failures *int, attempts *int) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if *failures > 0 {
				*failures--
				if stderr != nil {
					io.WriteString(stderr, "Preparing worktree (new branch 'x')\n")
				}
				return 255, nil
			}
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func TestGitWorktreeCreate_RetriesTransientAddFailure(t *testing.T) {
	root := initRetryRepo(t)
	prevRunner, prevSleep := gitRunner, worktreeAddRetrySleep
	t.Cleanup(func() { gitRunner, worktreeAddRetrySleep = prevRunner, prevSleep })
	var slept []time.Duration
	worktreeAddRetrySleep = func(d time.Duration) { slept = append(slept, d) }
	failures, attempts := 1, 0
	gitRunner = failingAddRunner(&failures, &attempts)

	wt, err := gitWorktree{}.Create(root, 7)
	if err != nil {
		t.Fatalf("Create must succeed on retry after ONE transient rc=255, got: %v", err)
	}
	if fi, statErr := os.Stat(wt); statErr != nil || !fi.IsDir() {
		t.Fatalf("worktree dir not created: %v", statErr)
	}
	if attempts != 2 {
		t.Fatalf("worktree add attempts = %d, want 2 (fail once, succeed once)", attempts)
	}
	if len(slept) != 1 {
		t.Fatalf("backoff sleeps = %v, want exactly one between the two attempts", slept)
	}
}

func TestGitWorktreeCreate_PersistentFailureStillFailsLoudly(t *testing.T) {
	root := initRetryRepo(t)
	prevRunner, prevSleep := gitRunner, worktreeAddRetrySleep
	t.Cleanup(func() { gitRunner, worktreeAddRetrySleep = prevRunner, prevSleep })
	worktreeAddRetrySleep = func(time.Duration) {}
	failures, attempts := 99, 0
	gitRunner = failingAddRunner(&failures, &attempts)

	_, err := gitWorktree{}.Create(root, 8)
	if err == nil {
		t.Fatal("persistent failure must still error — the fail-fast alarm chain downstream is correct and must be preserved")
	}
	if !strings.Contains(err.Error(), "rc=255") {
		t.Fatalf("final error must carry the git diagnosis, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("worktree add attempts = %d, want exactly 3 (bounded retry, never unbounded)", attempts)
	}
}
