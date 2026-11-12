package baseattr

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func GoTestRunner(env []string) Runner {
	return goTestRunner(lanebudget.Runner(sysexec.DefaultRunner), env)
}

func goTestRunner(run sysexec.RunFunc, env []string) Runner {
	return func(ctx context.Context, moduleDir string, r Reds) ([]Key, error) {
		stdout, stderr, code, err := sysexec.Capture(ctx, sysexec.WithEnv(run, env), moduleDir, "go", goTestArgs(ctx, r)...)
		if err != nil {
			return nil, fmt.Errorf("baseattr: go test did not start: %w", err)
		}
		return redKeys(r, ParseGoTest(stdout+"\n"+stderr), code)
	}
}

func redKeys(asked Reds, seen Reds, code int) ([]Key, error) {
	switch {
	case seen.Unnamed:
		return nil, fmt.Errorf("baseattr: go test exit %d with unnamed failures", code)
	case code != 0 && len(seen.Keys) == 0:
		return nil, fmt.Errorf("baseattr: go test exit %d with no failed test", code)
	}
	var red []Key
	for _, k := range seen.Keys {
		if slices.Contains(asked.Keys, k) {
			red = append(red, k)
		}
	}
	return red, nil
}

func goTestArgs(ctx context.Context, r Reds) []string {
	args := []string{"test", "-count=1"}
	if deadline, ok := ctx.Deadline(); ok {
		args = append(args, "-timeout", max(time.Until(deadline).Truncate(time.Second), time.Second).String())
	}
	args = append(args, r.Flags...)
	args = append(args, "-run", runPattern(r.Keys))
	return append(args, packagesOf(r.Keys)...)
}

func runPattern(keys []Key) string {
	var names []string
	for _, k := range keys {
		if q := regexp.QuoteMeta(k.Test); !slices.Contains(names, q) {
			names = append(names, q)
		}
	}
	return "^(" + strings.Join(names, "|") + ")$"
}

func packagesOf(keys []Key) []string {
	var pkgs []string
	for _, k := range keys {
		if !slices.Contains(pkgs, k.Package) {
			pkgs = append(pkgs, k.Package)
		}
	}
	return pkgs
}
