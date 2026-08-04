package gitexec

// worktree_retry_test.go — RED contract for cycle-1268 task
// `worktree-provisioning-retry-consolidate`.
//
// PR #401 (a497ffe1) proved that a transient `git worktree add` collision
// (rc=255, nothing on stderr but "Preparing worktree") under concurrent
// multi-lane provisioning must be retried, not treated as permanent — one
// collision left ActiveWorktree empty and cost the lane its whole cycle. That
// fix landed at exactly ONE call site (core.gitWorktree.Create). Three siblings
// still issue the bare, unretried add:
//
//	core.gitWorktree.CreateFrom          (continuation seeding, ADR-0076)
//	swarm.gitWorkerProvisioner.addWorktree (N concurrent workers — highest contention)
//	cmd/evolve runWorktreeCreate         (operator CLI)
//
// This file pins the SHARED helper the three adopt. gitexec is the home because
// it is the only package all three already depend on: swarm documents that it
// must not import core (provision.go:14-19), and `go list -deps ./cmd/evolve`
// confirms cmd/evolve already reaches gitexec — so no new import edge, and no
// cycle-644-shaped unsatisfiable pin.
//
// The retry knobs are passed as a value (WorktreeAddRetry), not read from
// exported mutable package globals: that keeps the loop single-sourced while
// letting core keep its existing worktreeAddAttempts/worktreeAddRetrySleep
// identifiers, so PR #401's worktree_retry_test.go stays green unmodified.

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// addFailRunner fails the first *failures `worktree add` invocations with the
// live incident's exact shape (rc=255 + "Preparing worktree" noise) and counts
// every attempt. Non-add git calls and post-failure attempts succeed silently —
// this package's helper is being tested, not git.
func addFailRunner(failures, attempts *int) sysexec.RunFunc {
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
		return 0, nil
	}
}

func TestAddWorktreeWithRetry_RetriesTransientFailure(t *testing.T) {
	failures, attempts := 1, 0
	var slept []time.Duration
	g := Git{Dir: "/repo", Exec: addFailRunner(&failures, &attempts)}

	_, _, code, err := g.AddWorktreeWithRetry(context.Background(),
		WorktreeAddRetry{Sleep: func(d time.Duration) { slept = append(slept, d) }},
		"-B", "cycle-9", "/base/cycle-9", "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("one transient rc=255 must be absorbed, got code=%d err=%v", code, err)
	}
	if attempts != 2 {
		t.Errorf("worktree add attempts = %d, want 2 (fail once, succeed once)", attempts)
	}
	if len(slept) != 1 {
		t.Errorf("backoff sleeps = %v, want exactly one between the two attempts", slept)
	}
}

func TestAddWorktreeWithRetry_BoundedThenSurfacesFinalFailure(t *testing.T) {
	failures, attempts := 99, 0
	var slept []time.Duration
	g := Git{Dir: "/repo", Exec: addFailRunner(&failures, &attempts)}

	_, stderr, code, err := g.AddWorktreeWithRetry(context.Background(),
		WorktreeAddRetry{Sleep: func(d time.Duration) { slept = append(slept, d) }},
		"-B", "cycle-9", "/base/cycle-9", "HEAD")

	// The downstream alarm chain (CB.2 fail-fast on an empty ActiveWorktree) is
	// CORRECT and must stay armed — a persistent failure still fails loudly with
	// the git diagnosis intact. Silencing it is the refuted PR #400.
	if code != 255 {
		t.Errorf("persistent failure must surface the final exit code, got code=%d err=%v", code, err)
	}
	if !strings.Contains(stderr, "Preparing worktree") {
		t.Errorf("final stderr must carry git's own diagnosis, got %q", stderr)
	}
	if attempts != DefaultWorktreeAddAttempts {
		t.Errorf("attempts = %d, want exactly DefaultWorktreeAddAttempts=%d (bounded, never unbounded)",
			attempts, DefaultWorktreeAddAttempts)
	}
	if len(slept) != DefaultWorktreeAddAttempts-1 {
		t.Errorf("backoff sleeps = %d, want %d (one between each pair of attempts, none after the last)",
			len(slept), DefaultWorktreeAddAttempts-1)
	}
}

// A clean provision is the overwhelmingly common case: it must cost exactly one
// git invocation and zero backoff. A helper that sleeps or re-runs on success
// would tax every cycle in the fleet to fix a rare collision.
func TestAddWorktreeWithRetry_CleanRunCostsOneAttemptAndNoSleep(t *testing.T) {
	failures, attempts := 0, 0
	slept := 0
	g := Git{Dir: "/repo", Exec: addFailRunner(&failures, &attempts)}

	_, _, code, err := g.AddWorktreeWithRetry(context.Background(),
		WorktreeAddRetry{Sleep: func(time.Duration) { slept++ }},
		"--detach", "/base/cycle-9", "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("clean add must succeed, got code=%d err=%v", code, err)
	}
	if attempts != 1 || slept != 0 {
		t.Errorf("clean add cost = %d attempt(s)/%d sleep(s), want 1/0", attempts, slept)
	}
}

// The zero value must be usable: a caller that supplies no knobs gets the
// default attempt bound and real time.Sleep, without a nil-func panic. Driven
// on the success path so the assertion costs no wall-clock backoff.
func TestAddWorktreeWithRetry_ZeroValueConfigUsesDefaults(t *testing.T) {
	failures, attempts := 0, 0
	g := Git{Dir: "/repo", Exec: addFailRunner(&failures, &attempts)}

	if _, _, code, err := g.AddWorktreeWithRetry(context.Background(), WorktreeAddRetry{},
		"-B", "cycle-9", "/base/cycle-9", "HEAD"); err != nil || code != 0 {
		t.Fatalf("zero-value WorktreeAddRetry must be usable, got code=%d err=%v", code, err)
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
	if DefaultWorktreeAddAttempts < 2 {
		t.Errorf("DefaultWorktreeAddAttempts = %d, want >= 2 — a bound of 1 is no retry at all",
			DefaultWorktreeAddAttempts)
	}
}

// The helper must prepend `worktree add` itself: callers pass only the add's
// own arguments, so no site can drift into a different subcommand while
// claiming the shared retry contract.
func TestAddWorktreeWithRetry_IssuesWorktreeAddArgv(t *testing.T) {
	var got []string
	g := Git{Dir: "/repo", Exec: func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		got = append([]string(nil), args...)
		return 0, nil
	}}

	if _, _, _, err := g.AddWorktreeWithRetry(context.Background(), WorktreeAddRetry{},
		"-B", "cycle-9", "/base/cycle-9", "HEAD"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := []string{"worktree", "add", "-B", "cycle-9", "/base/cycle-9", "HEAD"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("argv = %v, want %v", got, want)
	}
}
