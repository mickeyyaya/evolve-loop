package gitexec

import (
	"context"
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
	argv := append([]string{"worktree", "add"}, args...)
	return g.captureWithRetry(ctx, retryLoop{
		what: "worktree add", attempts: DefaultWorktreeAddAttempts,
		sleep: r.Sleep, onRetry: r.OnRetry, retryable: r.Retryable,
	}, argv)
}
