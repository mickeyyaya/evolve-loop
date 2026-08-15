//go:build acs

// Package cycle1477 materialises the cycle-1477 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - worktree-retry-diagnostic-integrity → the shared `git worktree add` retry
//     must keep BOTH the initiating and the terminal failure recoverable from
//     what it returns, must ANNOUNCE contention before it pays the backoff, must
//     pay NO backoff on a permanent failure, and must leave a RECOVERED success
//     free of retry noise.
//
// State of the work when these predicates were authored. The behaviour above
// landed in cycle-1474 (commit 07514fe8, reachable from this lane's base
// 18aa6f05): go/internal/gitexec/worktree.go now captures the first retryable
// failure and appends it only on terminal failure, and calls OnRetry before
// Sleep. Predicates 001-004 are therefore expected PRE-EXISTING GREEN — they are
// the durable contract for the criteria, not a RED bar, and the test-report
// records that honestly rather than manufacturing a false RED.
//
// 005 is this cycle's NEW coverage and the reason the set is not a copy of
// cycle-1474's. Cycle-1474 drove ONLY the gitexec seam directly; nothing pinned
// that a PRODUCTION provisioning caller surfaces git's own diagnostic, or that
// the newly-added retry-history decoration does not leak into a caller's error
// on a single-attempt permanent failure. 005 drives
// swarm.NewGitWorkerProvisioner — the real constructor the composition root
// uses, with the real git binary — and asserts on the error it returns.
//
// Predicate strategy — every predicate invokes the system under test in-process
// and asserts on returned values (the cycle-85 degenerate-predicate ban): no
// source greps, no `go test` subprocess, no whole-package sweep, no wall-clock
// bound, no literal PID, no bare `git` against process cwd.
package cycle1477

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// mixedFailRunner scripts the recorded two-failure sequence: attempt 1 is the
// live collision shape (rc=255, EMPTY stderr — it carries no diagnosis of its
// own, so its exit code is the only evidence it happened), attempt 2 is the
// permanent rc=128 that first failure caused.
func mixedFailRunner(attempts *int) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if *attempts == 1 {
				return 255, nil
			}
			if stderr != nil {
				io.WriteString(stderr, "fatal: '/base/lane' already exists\n")
			}
			return 128, nil
		}
		return 0, nil
	}
}

// oneTransientRunner fails the first `worktree add` with the live collision
// shape and succeeds on the re-attempt.
func oneTransientRunner(attempts *int) sysexec.RunFunc {
	return func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if *attempts == 1 {
				if stderr != nil {
					io.WriteString(stderr, "Preparing worktree (new branch 'lane')\n")
				}
				return 255, nil
			}
		}
		return 0, nil
	}
}

// TestC1477_001_RetryPreservesInitiatingFailure is the crux criterion: a
// transient rc=255 followed by a DIFFERENT terminal failure must leave both
// recoverable. Losing the initiating rc=255 reports a plain path collision that
// never happened; losing the terminal code/stderr disarms the downstream
// fail-fast (the refuted PR #400 is the record of that cost).
func TestC1477_001_RetryPreservesInitiatingFailure(t *testing.T) {
	attempts := 0
	g := gitexec.Git{Dir: t.TempDir(), Exec: mixedFailRunner(&attempts)}

	_, stderr, code, _ := g.AddWorktreeWithRetry(context.Background(),
		gitexec.WorktreeAddRetry{
			Sleep:     func(time.Duration) {},
			Retryable: gitexec.RetryableWorktreeAddFailure,
		},
		"-B", "lane", filepath.Join(t.TempDir(), "wt"), "HEAD")

	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2 (transient rc=255, then the permanent rc=128 that ends the loop)", attempts)
	}
	if code != 128 {
		t.Errorf("exit code=%d, want the FINAL attempt's 128 — the terminal failure is what the caller fails on", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("terminal diagnostic lost git's own final stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "255") {
		t.Errorf("terminal diagnostic does not preserve the INITIATING rc=255 failure — a transient collision is being reported as a plain path collision: %q", stderr)
	}
}

// TestC1477_002_RetryAnnouncesBeforeBackoff pins the documented order. OnRetry
// exists so a caller can announce contention WHILE it is happening; announcing
// after the ladder leaves a lane stuck inside the 2s/4s window silent.
func TestC1477_002_RetryAnnouncesBeforeBackoff(t *testing.T) {
	attempts := 0
	var events []string
	g := gitexec.Git{Dir: t.TempDir(), Exec: oneTransientRunner(&attempts)}

	_, _, code, err := g.AddWorktreeWithRetry(context.Background(),
		gitexec.WorktreeAddRetry{
			Sleep:   func(time.Duration) { events = append(events, "sleep") },
			OnRetry: func(_, _, _ int, _ string) { events = append(events, "retry") },
		},
		"-B", "lane", filepath.Join(t.TempDir(), "wt"), "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("one transient failure must still be absorbed, got code=%d err=%v", code, err)
	}
	if len(events) != 2 || events[0] != "retry" || events[1] != "sleep" {
		t.Errorf("callback order=%v, want [retry sleep] — announce the contention, THEN pay the backoff", events)
	}
}

// TestC1477_003_PermanentFailureSkipsBackoff is the regression guard the task
// must not break: a classified-permanent failure costs ONE attempt, zero
// backoff and zero announcements, while the exit code and git's stderr survive
// intact. This is the criterion the diagnostic change is most likely to
// regress — decorating the return path is exactly where an extra attempt or a
// spurious announcement gets introduced.
func TestC1477_003_PermanentFailureSkipsBackoff(t *testing.T) {
	attempts, sleeps, announces := 0, 0, 0
	permanent := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			attempts++
			if stderr != nil {
				io.WriteString(stderr, "fatal: not a git repository (or any of the parent directories): .git\n")
			}
			return 128, nil
		}
		return 0, nil
	}
	g := gitexec.Git{Dir: t.TempDir(), Exec: permanent}

	_, stderr, code, _ := g.AddWorktreeWithRetry(context.Background(),
		gitexec.WorktreeAddRetry{
			Sleep:     func(time.Duration) { sleeps++ },
			OnRetry:   func(_, _, _ int, _ string) { announces++ },
			Retryable: gitexec.RetryableWorktreeAddFailure,
		},
		"-B", "lane", filepath.Join(t.TempDir(), "wt"), "HEAD")

	if attempts != 1 {
		t.Errorf("attempts=%d, want 1 — a permanent failure must not buy the retry ladder", attempts)
	}
	if sleeps != 0 {
		t.Errorf("sleeps=%d, want 0 — backoff on a permanent condition is pure latency", sleeps)
	}
	if announces != 0 {
		t.Errorf("announcements=%d, want 0 — announcing contention that was never classified is how a permanent rc=128 got logged as contention", announces)
	}
	if code != 128 || !strings.Contains(stderr, "not a git repository") {
		t.Errorf("permanent failure lost its diagnostic: code=%d stderr=%q", code, stderr)
	}
}

// TestC1477_004_RecoveredSuccessCarriesNoRetryHistory is the anti-gaming axis
// for 001: preserving evidence must be CONDITIONAL on failing. An implementation
// that decorates its return unconditionally would satisfy 001 while polluting
// every successful provision's stderr — and every caller renders that stderr.
func TestC1477_004_RecoveredSuccessCarriesNoRetryHistory(t *testing.T) {
	attempts := 0
	g := gitexec.Git{Dir: t.TempDir(), Exec: oneTransientRunner(&attempts)}

	_, stderr, code, err := g.AddWorktreeWithRetry(context.Background(),
		gitexec.WorktreeAddRetry{Sleep: func(time.Duration) {}},
		"-B", "lane", filepath.Join(t.TempDir(), "wt"), "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("transient failure then success must return success, got code=%d err=%v", code, err)
	}
	if attempts != 2 {
		t.Fatalf("attempts=%d, want 2 — this predicate is not exercising the absorbed-retry path", attempts)
	}
	if strings.Contains(stderr, "255") || strings.Contains(stderr, "initial worktree add failure") {
		t.Errorf("a SUCCEEDING add carries the absorbed attempt's failure noise: %q", stderr)
	}
}

// TestC1477_005_ProductionProvisionerSurfacesGitStderrWithoutFabricatedHistory
// is this cycle's WIRING PROOF and its new coverage over cycle-1474.
//
// Cycle-1474 pinned the helper in isolation; nothing pinned that a real
// provisioning caller renders what the helper returns. This drives the
// production constructor swarm.NewGitWorkerProvisioner (the one the composition
// root calls) with the REAL git binary against a directory that is not a
// repository, so the shared classifier fires on a real rc=128 and the real error
// path runs. Two things must hold at the caller:
//
//   - git's OWN stderr reaches the operator through the shared helper (the
//     reason the operator path was routed through gitexec at all: a raw
//     exec.Command could only ever report "exit status 255"), and
//   - the retry-history decoration added for 001 does NOT leak onto a
//     single-attempt permanent failure — the caller-visible negative axis that
//     004 only covers on the success path.
//
// Deterministic by construction: a non-repository parent yields the same rc=128
// "not a git repository" on every platform, one attempt, no backoff, no clock.
func TestC1477_005_ProductionProvisionerSurfacesGitStderrWithoutFabricatedHistory(t *testing.T) {
	p := swarm.NewGitWorkerProvisioner(nil, "")

	// t.TempDir() is outside any checkout, so real git refuses with the
	// permanent rc=128 the shared classifier must recognise.
	_, err := p.CreateWorker(context.Background(), t.TempDir(), 1477, "w1", "")
	if err == nil {
		t.Fatal("provisioning a worker outside a git repository returned no error — the failure never reached the caller")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a git repository") {
		t.Errorf("the production provisioner's error does not carry git's own stderr: %q", msg)
	}
	if strings.Contains(msg, "initial worktree add failure") {
		t.Errorf("a single-attempt PERMANENT failure was decorated with retry history that never happened: %q", msg)
	}
}
