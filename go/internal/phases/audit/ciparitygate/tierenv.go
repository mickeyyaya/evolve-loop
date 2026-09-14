package ciparitygate

import (
	"context"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// tierEnvAllowlist is the minimal environment the tier subprocess keeps —
// what a clean CI shell provides. Everything else (EVOLVE_*, BRIDGE_*,
// tmux/session vars) is the lane's runtime state and must not reach
// env-sensitive integration tests.
var tierEnvAllowlist = []string{
	// the shell
	"PATH", "HOME", "TMPDIR", "USER", "SHELL",

	// the toolchain
	"GOROOT", "GOPATH", "GOCACHE", "GOMODCACHE", "GOFLAGS", "GOTOOLCHAIN", "CC",
}

func cleanEnv() []string {
	env := make([]string, 0, len(tierEnvAllowlist))
	for _, k := range tierEnvAllowlist {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return env
}

// scrubbedRun wraps a sysexec.RunFunc so every invocation carries the scrubbed
// allowlist env — captured ONCE, here — instead of whatever env the caller
// passes (nil would inherit the lane's full os.Environ()).
func scrubbedRun(run sysexec.RunFunc) sysexec.RunFunc {
	clean := cleanEnv()
	return func(ctx context.Context, name, dir string, args, _ []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		return run(ctx, name, dir, args, clean, stdin, stdout, stderr)
	}
}
