package swarm

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// swarmAddFailRunner fails the first *failures worktree adds with the transient shape (rc=255, only "Preparing worktree" on stderr).
func swarmAddFailRunner(failures, attempts *int) sysexec.RunFunc {
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

func retryProvisioner(t *testing.T, failures, attempts *int, slept *[]time.Duration) gitWorkerProvisioner {
	t.Helper()
	run := swarmAddFailRunner(failures, attempts)
	return gitWorkerProvisioner{
		baseOverride: t.TempDir(),
		newGit:       func(dir string) gitexec.Git { return gitexec.Git{Dir: dir, Exec: run} },
		retry:        gitexec.WorktreeAddRetry{Sleep: func(d time.Duration) { *slept = append(*slept, d) }},
	}
}

func TestSwarmCreateWorker_RetriesTransientAddFailure(t *testing.T) {
	failures, attempts := 1, 0
	var slept []time.Duration
	p := retryProvisioner(t, &failures, &attempts, &slept)

	if _, err := p.CreateWorker(context.Background(), "/repo", 9, "w0", "cycle-9-integration"); err != nil {
		t.Fatalf("CreateWorker must absorb ONE transient rc=255, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("worktree add attempts = %d, want 2 (fail once, succeed once)", attempts)
	}
	if len(slept) != 1 {
		t.Errorf("backoff sleeps = %v, want exactly one between the two attempts", slept)
	}
}

func TestSwarmCreateIntegration_RetriesTransientAddFailure(t *testing.T) {
	failures, attempts := 1, 0
	var slept []time.Duration
	p := retryProvisioner(t, &failures, &attempts, &slept)

	if _, err := p.CreateIntegration(context.Background(), "/repo", 9); err != nil {
		t.Fatalf("CreateIntegration must absorb ONE transient rc=255, got: %v", err)
	}
	if attempts != 2 {
		t.Errorf("worktree add attempts = %d, want 2 (fail once, succeed once)", attempts)
	}
	if len(slept) != 1 {
		t.Errorf("backoff sleeps = %v, want exactly one between the two attempts", slept)
	}
}

func TestSwarmCreateWorker_PersistentFailureStillFailsLoudly(t *testing.T) {
	failures, attempts := 99, 0
	var slept []time.Duration
	p := retryProvisioner(t, &failures, &attempts, &slept)

	_, err := p.CreateWorker(context.Background(), "/repo", 9, "w0", "cycle-9-integration")
	if err == nil {
		t.Fatal("persistent failure must still error — a swarm worker that silently gets no worktree is the failure this alarm exists to catch")
	}
	if !strings.Contains(err.Error(), "255") {
		t.Errorf("final error must carry git's exit code, got: %v", err)
	}
	if attempts != gitexec.DefaultWorktreeAddAttempts {
		t.Errorf("attempts = %d, want exactly DefaultWorktreeAddAttempts=%d — the swarm must share ONE bound with core, not a private copy",
			attempts, gitexec.DefaultWorktreeAddAttempts)
	}
}

func TestSwarmCreateWorker_CleanRunCostsOneAttemptAndNoSleep(t *testing.T) {
	failures, attempts := 0, 0
	var slept []time.Duration
	p := retryProvisioner(t, &failures, &attempts, &slept)

	if _, err := p.CreateWorker(context.Background(), "/repo", 9, "w0", "cycle-9-integration"); err != nil {
		t.Fatalf("clean CreateWorker must succeed: %v", err)
	}
	if attempts != 1 || len(slept) != 0 {
		t.Errorf("clean CreateWorker cost = %d attempt(s)/%d sleep(s), want 1/0", attempts, len(slept))
	}
}
