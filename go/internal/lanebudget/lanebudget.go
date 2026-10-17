// Package lanebudget gives each go test that a fleet lane runs one share of the host CPUs.
// See docs/architecture/packages/internal-lanebudget.md.
package lanebudget

import (
	"context"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var hostCPUs = runtime.NumCPU

func Procs(hostCPUs, width int) int {
	if width < 1 {
		return 0
	}
	return max(1, hostCPUs/width)
}

func Current() int {
	width, err := strconv.Atoi(strings.TrimSpace(os.Getenv(ipcenv.FleetWidthKey)))
	if err != nil {
		return 0
	}
	return Procs(hostCPUs(), width)
}

func Args(args []string) []string {
	procs := Current()
	if procs == 0 || !isGoTest(args) {
		return args
	}
	value := strconv.Itoa(procs)
	out := []string{args[0]}
	for _, flag := range []string{"-p", "-parallel"} {
		if !hasFlag(args[1:], flag) {
			out = append(out, flag, value)
		}
	}
	return append(out, args[1:]...)
}

func Env(env []string) []string {
	procs := Current()
	if procs == 0 {
		return env
	}
	if env == nil {
		env = os.Environ()
	}
	return append(slices.Clip(env), "GOMAXPROCS="+strconv.Itoa(procs))
}

func Runner(run sysexec.RunFunc) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "go" && isGoTest(args) {
			args, env = Args(args), Env(env)
		}
		return run(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func isGoTest(args []string) bool {
	return len(args) > 0 && args[0] == "test"
}

func hasFlag(args []string, flag string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		return arg == flag || strings.HasPrefix(arg, flag+"=")
	})
}
