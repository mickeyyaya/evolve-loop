//go:build !unix

package cliupdate

import (
	"context"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func GroupRunner(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
}
