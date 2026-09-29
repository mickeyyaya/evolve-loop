package ciparitygate

import (
	"context"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// scrubbedRun wraps a sysexec.RunFunc so every invocation carries the scrubbed
// allowlist env — captured ONCE, here — instead of whatever env the caller
// passes (nil would inherit the lane's full os.Environ()).
func scrubbedRun(run sysexec.RunFunc) sysexec.RunFunc {
	clean := ciparity.CIEnv(os.Environ())
	return func(ctx context.Context, name, dir string, args, _ []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return run(ctx, name, dir, args, clean, stdin, stdout, stderr)
	}
}
