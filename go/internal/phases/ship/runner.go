package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func defaultNow() Now {
	t := time.Now().UTC()
	return Now{
		Unix:    t.Unix(),
		RFC3339: t.Format(time.RFC3339),
	}
}

func (o *Options) runner() CmdRunner {
	if o.Runner == nil {
		return sysexec.DefaultRunner
	}
	return o.Runner
}

func (o *Options) run(ctx context.Context, name string, args []string, stdout, stderr io.Writer) (int, error) {
	if name == "git" {
		args = scopeStatusRead(ctx, args)
	}
	return o.runner()(ctx, name, o.ProjectRoot, args, os.Environ(), nil, stdout, stderr)
}

func (o *Options) runStdin(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return o.runner()(ctx, name, o.ProjectRoot, args, os.Environ(), stdin, stdout, stderr)
}

func (o *Options) rel(base, target string) (string, error) {
	if o.relPath != nil {
		return o.relPath(base, target)
	}
	return filepath.Rel(base, target)
}

var execRunner CmdRunner = sysexec.DefaultRunner
