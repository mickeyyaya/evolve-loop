package gitexec

import (
	"context"
	"fmt"
	"time"
)

type retryLoop struct {
	what      string
	attempts  int
	sleep     func(time.Duration)
	onRetry   func(attempt, attempts, code int, stderr string)
	retryable func(code int, stderr string) bool
}

func (g Git) captureWithRetry(ctx context.Context, l retryLoop, argv []string) (stdout, stderr string, exitCode int, err error) {
	sleep := l.sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	var firstFailure string
	for attempt := 0; attempt < l.attempts; attempt++ {
		stdout, stderr, exitCode, err = g.Capture(ctx, argv...)
		if err == nil && exitCode == 0 {
			return stdout, stderr, exitCode, nil
		}
		isLastAttempt := attempt == l.attempts-1
		if isLastAttempt {
			break
		}
		isPermanentFailure := l.retryable != nil && !l.retryable(exitCode, stderr)
		if isPermanentFailure {
			break
		}
		if firstFailure == "" {
			firstFailure = fmt.Sprintf("initial %s failure (rc=%d): %s", l.what, exitCode, stderr)
		}
		if l.onRetry != nil {
			l.onRetry(attempt+1, l.attempts, exitCode, stderr)
		}
		sleep(time.Duration(attempt+1) * 2 * time.Second)
	}
	if firstFailure != "" {
		stderr = firstFailure + "\nfinal " + l.what + " failure: " + stderr
	}
	return stdout, stderr, exitCode, err
}
