package ship

import (
	"errors"
	"io"
	"syscall"
)

// captureWithEBADFRetry runs fn; on a transient EBADF or closed-pipe error it
// retries exactly once. A persistent EBADF (fails on both attempts) is returned
// as-is, preserving the error chain for the caller. Non-EBADF errors pass
// through on the first attempt with zero retries so genuine git failures are
// never masked. Test-infra only — never called from production ship/ code.
func captureWithEBADFRetry(fn func() ([]byte, error)) ([]byte, error) {
	out, err := fn()
	if err == nil || !isEBADFLike(err) {
		return out, err
	}
	return fn()
}

func isEBADFLike(err error) bool {
	return errors.Is(err, syscall.EBADF) || errors.Is(err, io.ErrClosedPipe)
}
