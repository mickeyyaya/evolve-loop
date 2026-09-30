//go:build acs

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

func TestC1477_005_ProductionProvisionerSurfacesGitStderrWithoutFabricatedHistory(t *testing.T) {
	p := swarm.NewGitWorkerProvisioner(nil, "")

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
