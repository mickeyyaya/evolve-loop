package ship

import (
	"errors"
	"io"
	"syscall"
)

// captureWithEBADFRetry runs fn and retries once on a transient EBADF or
// closed-pipe error; a persistent EBADF returns as-is, and non-EBADF errors
// pass straight through. Test-infra only — never called from production ship code.
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
