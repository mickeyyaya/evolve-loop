package proctree

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type ArgsReader func(pid int) ([]string, map[string]string, error)

var psArgs = []string{"-A", "-o", "pid=,ppid=,pgid=,uid=,lstart=,comm="}

func NewLister(run sysexec.RunFunc, read ArgsReader, uid int) Lister {
	return func(ctx context.Context) ([]Process, error) {
		var out, errOut strings.Builder
		env := append(os.Environ(), "LC_ALL=C")
		code, err := run(ctx, "ps", "", psArgs, env, nil, &out, &errOut)
		if err != nil {
			return nil, fmt.Errorf("ps: %w", err)
		}
		if code != 0 {
			return nil, fmt.Errorf("ps: exit %d: %s", code, strings.TrimSpace(errOut.String()))
		}
		return withArgs(parsePS(out.String(), uid), read), nil
	}
}

func withArgs(table []Process, read ArgsReader) []Process {
	out := make([]Process, 0, len(table))
	for _, p := range table {
		if args, env, err := read(p.Pid); err == nil {
			p.Args, p.Env = args, env
		}
		out = append(out, p)
	}
	return out
}

func ExecLister() Lister {
	return NewLister(sysexec.DefaultRunner, readProcArgs, os.Getuid())
}
