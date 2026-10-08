package ship

import "github.com/mickeyyaya/evolve-loop/go/internal/gittest"

func captureWithEBADFRetry(fn func() ([]byte, error)) ([]byte, error) {
	return gittest.CaptureWithEBADFRetry(fn)
}
