package proctree

import (
	"context"
	"os"
)

type argsReader func(pid int) ([]string, map[string]string, error)

type tableReader func(uid int) ([]Process, error)

func newLister(table tableReader, read argsReader, uid int) Lister {
	return func(context.Context) ([]Process, error) {
		rows, err := table(uid)
		if err != nil {
			return nil, err
		}
		return withArgs(rows, read), nil
	}
}

func withArgs(table []Process, read argsReader) []Process {
	out := make([]Process, 0, len(table))
	for _, p := range table {
		if args, env, err := read(p.Pid); err == nil {
			p.Args, p.Env = args, env
			p.Comm = commFrom(p)
		}
		out = append(out, p)
	}
	return out
}

func ExecLister() Lister {
	return newLister(readTable, readProcArgs, os.Getuid())
}
