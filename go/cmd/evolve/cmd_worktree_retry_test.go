package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func cliAddFailRunner(failures, attempts *int) sysexec.RunFunc {
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

// installCLIRetryFakes swaps in the injectable git runner + a sleep recorder
// and restores both, so the operator CLI can be driven without touching a real
// repo or paying real backoff.
func installCLIRetryFakes(t *testing.T, failures, attempts *int, slept *[]time.Duration) {
	t.Helper()
	prevRunner, prevRetry := worktreeGitRunner, worktreeAddRetry
	t.Cleanup(func() { worktreeGitRunner, worktreeAddRetry = prevRunner, prevRetry })
	worktreeGitRunner = cliAddFailRunner(failures, attempts)
	worktreeAddRetry = gitexec.WorktreeAddRetry{Sleep: func(d time.Duration) { *slept = append(*slept, d) }}
}

func TestRunWorktreeCreate_RetriesTransientAddFailure(t *testing.T) {
	failures, attempts := 1, 0
	var slept []time.Duration
	installCLIRetryFakes(t, &failures, &attempts, &slept)

	var out, errb bytes.Buffer
	code := runWorktreeCreate([]string{"-cycle", "9", "-project-root", t.TempDir(), "-base", t.TempDir()}, &out, &errb)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 — one transient rc=255 must be absorbed. stderr:\n%s", code, errb.String())
	}
	if attempts != 2 {
		t.Errorf("worktree add attempts = %d, want 2 (fail once, succeed once)", attempts)
	}
	if len(slept) != 1 {
		t.Errorf("backoff sleeps = %v, want exactly one between the two attempts", slept)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Errorf("successful create must still print the worktree path to stdout (the operator contract), got %q", out.String())
	}
}

func TestRunWorktreeCreate_PersistentFailureStillFailsLoudly(t *testing.T) {
	failures, attempts := 99, 0
	var slept []time.Duration
	installCLIRetryFakes(t, &failures, &attempts, &slept)

	var out, errb bytes.Buffer
	code := runWorktreeCreate([]string{"-cycle", "9", "-project-root", t.TempDir(), "-base", t.TempDir()}, &out, &errb)

	if code == 0 {
		t.Fatal("persistent failure must exit non-zero — an operator handed a zero exit and no worktree is the silent failure this closes")
	}
	if attempts != gitexec.DefaultWorktreeAddAttempts {
		t.Errorf("attempts = %d, want exactly DefaultWorktreeAddAttempts=%d — the CLI must share ONE bound with core and swarm",
			attempts, gitexec.DefaultWorktreeAddAttempts)
	}
	diag := errb.String()
	if !strings.Contains(diag, "255") || !strings.Contains(diag, "Preparing worktree") {
		t.Errorf("stderr must carry git's exit code AND its own message, got:\n%s", diag)
	}
}

func TestRunWorktreeCreate_CleanRunCostsOneAttemptAndNoSleep(t *testing.T) {
	failures, attempts := 0, 0
	var slept []time.Duration
	installCLIRetryFakes(t, &failures, &attempts, &slept)

	var out, errb bytes.Buffer
	if code := runWorktreeCreate([]string{"-cycle", "9", "-project-root", t.TempDir(), "-base", t.TempDir()}, &out, &errb); code != 0 {
		t.Fatalf("exit = %d, want 0. stderr:\n%s", code, errb.String())
	}
	if attempts != 1 || len(slept) != 0 {
		t.Errorf("clean create cost = %d attempt(s)/%d sleep(s), want 1/0", attempts, len(slept))
	}
}
