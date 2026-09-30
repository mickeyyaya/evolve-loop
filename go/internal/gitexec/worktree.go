package gitexec

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const DefaultWorktreeAddAttempts = 3

type WorktreeAddRetry struct {
	Sleep func(time.Duration)

	OnRetry func(attempt, attempts, code int, stderr string)

	Retryable func(code int, stderr string) bool
}

var permanentWorktreeAddStderr = []string{
	"not a git repository",
	"is already checked out",
	"already exists",
}

func RetryableWorktreeAddFailure(code int, stderr string) bool {
	low := strings.ToLower(stderr)
	for _, marker := range permanentWorktreeAddStderr {
		if strings.Contains(low, marker) {
			return false
		}
	}
	return true
}

func (g Git) AddWorktreeWithRetry(ctx context.Context, r WorktreeAddRetry, args ...string) (stdout, stderr string, exitCode int, err error) {
	sleep := r.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	argv := append([]string{"worktree", "add"}, args...)
	var firstFailure string
	for attempt := 0; attempt < DefaultWorktreeAddAttempts; attempt++ {
		stdout, stderr, exitCode, err = g.Capture(ctx, argv...)
		if err == nil && exitCode == 0 {
			return stdout, stderr, exitCode, nil
		}
		isLastAttempt := attempt == DefaultWorktreeAddAttempts-1
		if isLastAttempt {
			break
		}
		isPermanentFailure := r.Retryable != nil && !r.Retryable(exitCode, stderr)
		if isPermanentFailure {
			break
		}
		if firstFailure == "" {
			firstFailure = fmt.Sprintf("initial worktree add failure (rc=%d): %s", exitCode, stderr)
		}
		if r.OnRetry != nil {
			r.OnRetry(attempt+1, DefaultWorktreeAddAttempts, exitCode, stderr)
		}
		sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	if firstFailure != "" {
		stderr = firstFailure + "\nfinal worktree add failure: " + stderr
	}
	return stdout, stderr, exitCode, err
}
