package gitexec

import (
	"context"
	"strings"
	"time"
)

const DefaultFetchAttempts = 3

type FetchRetry struct {
	Attempts int

	Sleep func(time.Duration)

	OnRetry func(attempt, attempts, code int, stderr string)
}

var refLockContentionStderr = []string{
	"cannot lock ref",
	"unable to update local ref",
}

func RetryableFetchFailure(_ int, stderr string) bool {
	low := strings.ToLower(stderr)
	for _, marker := range refLockContentionStderr {
		if strings.Contains(low, marker) {
			return true
		}
	}
	return false
}

func (g Git) FetchOriginBranch(ctx context.Context, r FetchRetry, branch string) (stdout, stderr string, exitCode int, err error) {
	attempts := r.Attempts
	if attempts <= 0 {
		attempts = DefaultFetchAttempts
	}
	refspec := "+refs/heads/" + branch + ":refs/remotes/origin/" + branch
	return g.captureWithRetry(ctx, retryLoop{
		what: "fetch", attempts: attempts,
		sleep: r.Sleep, onRetry: r.OnRetry, retryable: RetryableFetchFailure,
	}, []string{"fetch", "origin", refspec})
}
