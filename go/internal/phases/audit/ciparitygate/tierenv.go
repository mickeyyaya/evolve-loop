package ciparitygate

import (
	"context"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func scrubbedRun(run sysexec.RunFunc) sysexec.RunFunc {
	clean := ciparity.CIEnv(os.Environ())
	return func(ctx context.Context, name, dir string, args, _ []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return run(ctx, name, dir, args, clean, stdin, stdout, stderr)
	}
}
