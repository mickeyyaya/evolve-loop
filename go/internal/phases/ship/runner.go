// runner.go — clock helper + the Options command-execution helpers for the
// native ship path. CmdRunner (= sysexec.RunFunc) is defined in ship.go.
package ship

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/sysexec"
)

func defaultNow() Now {
	t := time.Now().UTC()
	return Now{
		Unix:    t.Unix(),
		RFC3339: t.Format(time.RFC3339),
	}
}

// runner resolves the command-execution seam, defaulting to
// sysexec.DefaultRunner when Options.Runner is unset. Centralizing this nil
// fallback here lets every helper (and direct callers like committedBinSHA in
// tests) run safely without a per-call nil-guard — production Run() still sets
// Runner explicitly; only direct-helper-call tests rely on this default.
func (o *Options) runner() CmdRunner {
	if o.Runner == nil {
		return sysexec.DefaultRunner
	}
	return o.Runner
}

// run executes a command rooted at ProjectRoot with the inherited environment
// and no stdin, streaming to the given writers. It centralizes the
// (cwd=ProjectRoot, env=os.Environ(), stdin=nil) boilerplate every ship git/gh
// call repeated; worktree-scoped calls still pass `-C <dir>` in args.
func (o *Options) run(ctx context.Context, name string, args []string, stdout, stderr io.Writer) (int, error) {
	return o.runner()(ctx, name, o.ProjectRoot, args, os.Environ(), nil, stdout, stderr)
}

// runStdin is run with a piped stdin (e.g. `gh release create --notes-file -`).
func (o *Options) runStdin(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return o.runner()(ctx, name, o.ProjectRoot, args, os.Environ(), stdin, stdout, stderr)
}

// execRunner is the package-level production runner seam. It is an alias for
// sysexec.DefaultRunner with the CmdRunner (= sysexec.RunFunc) signature:
// (ctx, name, dir, args, env, stdin, stdout, stderr). Tests inject this as
// Options.Runner for integration-style tests that invoke real git.
var execRunner CmdRunner = sysexec.DefaultRunner
