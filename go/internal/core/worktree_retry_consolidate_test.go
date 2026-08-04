package core

// worktree_retry_consolidate_test.go — RED contract for cycle-1268 task
// `worktree-provisioning-retry-consolidate`, adoption site #1:
// gitWorktree.CreateFrom.
//
// CreateFrom is the ADR-0076 continuation-seeding path — the one that
// provisions the worktree for a cycle RESUMING salvaged work — and it issues a
// bare, unretried `git worktree add` (worktree.go:208). A transient lock
// collision there costs the continuation its cycle in exactly the way PR #401
// fixed for Create, with the added insult that the salvaged work is what is
// being dropped on the floor.
//
// Fixtures (initRetryRepo, failingAddRunner) and the sleep no-op init() are
// shared with worktree_retry_test.go — PR #401's file, which must stay green
// and unmodified: "existing tests staying green" is part of this task's
// acceptance criteria, so the consolidation may not break core's
// worktreeAddAttempts / worktreeAddRetrySleep identifiers.

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestGitWorktreeCreateFrom_RetriesTransientAddFailure(t *testing.T) {
	root := initRetryRepo(t)
	prevRunner, prevSleep := gitRunner, worktreeAddRetrySleep
	t.Cleanup(func() { gitRunner, worktreeAddRetrySleep = prevRunner, prevSleep })
	var slept []time.Duration
	worktreeAddRetrySleep = func(d time.Duration) { slept = append(slept, d) }
	failures, attempts := 1, 0
	gitRunner = failingAddRunner(&failures, &attempts)

	wt, err := gitWorktree{}.CreateFrom(root, 11, "HEAD")
	if err != nil {
		t.Fatalf("CreateFrom must succeed on retry after ONE transient rc=255, got: %v", err)
	}
	if fi, statErr := os.Stat(wt); statErr != nil || !fi.IsDir() {
		t.Fatalf("worktree dir not created: %v", statErr)
	}
	if attempts != 2 {
		t.Errorf("worktree add attempts = %d, want 2 (fail once, succeed once)", attempts)
	}
	if len(slept) != 1 {
		t.Errorf("backoff sleeps = %v, want exactly one between the two attempts", slept)
	}
}

func TestGitWorktreeCreateFrom_PersistentFailureStillFailsLoudly(t *testing.T) {
	root := initRetryRepo(t)
	prevRunner, prevSleep := gitRunner, worktreeAddRetrySleep
	t.Cleanup(func() { gitRunner, worktreeAddRetrySleep = prevRunner, prevSleep })
	worktreeAddRetrySleep = func(time.Duration) {}
	failures, attempts := 99, 0
	gitRunner = failingAddRunner(&failures, &attempts)

	if _, err := (gitWorktree{}).CreateFrom(root, 12, "HEAD"); err == nil {
		t.Fatal("persistent failure must still error — the downstream fail-fast alarm chain is correct and must stay armed")
	} else if !strings.Contains(err.Error(), "rc=255") {
		t.Errorf("final error must carry the git diagnosis, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("worktree add attempts = %d, want exactly 3 (bounded retry, never unbounded)", attempts)
	}
}

// A continuation that provisions cleanly must pay nothing for the retry
// capability — one git invocation, zero backoff.
func TestGitWorktreeCreateFrom_CleanRunCostsOneAttemptAndNoSleep(t *testing.T) {
	root := initRetryRepo(t)
	prevRunner, prevSleep := gitRunner, worktreeAddRetrySleep
	t.Cleanup(func() { gitRunner, worktreeAddRetrySleep = prevRunner, prevSleep })
	slept := 0
	worktreeAddRetrySleep = func(time.Duration) { slept++ }
	failures, attempts := 0, 0
	gitRunner = failingAddRunner(&failures, &attempts)

	if _, err := (gitWorktree{}).CreateFrom(root, 13, "HEAD"); err != nil {
		t.Fatalf("clean CreateFrom must succeed: %v", err)
	}
	if attempts != 1 || slept != 0 {
		t.Errorf("clean CreateFrom cost = %d attempt(s)/%d sleep(s), want 1/0", attempts, slept)
	}
}
