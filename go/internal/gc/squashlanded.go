package gc

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const squashPathspecChunk = 100

const literalPathspec = ":(literal)"

type squashResult struct {
	Landed bool
	NoNet  bool
	Ahead  int
}

func (r squashResult) landing() landing {
	switch {
	case !r.Landed:
		return landingNone
	case r.NoNet && r.Ahead == 0:
		return landingNoNet
	case r.NoNet:
		return landingNone
	}
	return landingSquash
}

func (r squashResult) deletable() bool {
	return r.landing() == landingSquash || r.landing() == landingNoNet
}

const diffSubmodulesConfig = "diff.ignoreSubmodules=none"

func branchLandedBySquash(ctx context.Context, g gitexec.Git, target, branch string) (squashResult, error) {
	ref := "refs/heads/" + branch
	base, stderr, code, err := g.Capture(ctx, "merge-base", target, ref)
	base = strings.TrimSpace(base)
	switch {
	case err != nil || code > 1:
		return squashResult{}, fmt.Errorf("gc: git merge-base %s %s: exit %d err=%v: %s", target, ref, code, err, strings.TrimSpace(stderr))
	case code == 1 || base == "":
		return squashResult{}, nil
	}
	out, err := g.Output(ctx, "-c", diffSubmodulesConfig, "diff", "--name-only", "-z", "--no-renames", base, ref)
	if err != nil {
		return squashResult{}, fmt.Errorf("gc: git diff %s..%s: %w", base, ref, err)
	}
	paths := splitNul(out)
	if len(paths) == 0 {
		return noNetChanges(ctx, g, target, ref)
	}
	landed, err := squashPathsEqual(ctx, g, target, ref, paths)
	return squashResult{Landed: landed}, err
}

func noNetChanges(ctx context.Context, g gitexec.Git, target, ref string) (squashResult, error) {
	out, err := g.Output(ctx, "rev-list", "--count", target+".."+ref)
	if err != nil {
		return squashResult{}, fmt.Errorf("gc: git rev-list --count %s..%s: %w", target, ref, err)
	}
	ahead, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return squashResult{}, fmt.Errorf("gc: git rev-list --count %s..%s answered %q: %w", target, ref, out, err)
	}
	return squashResult{Landed: true, NoNet: true, Ahead: ahead}, nil
}

func splitNul(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "\x00") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func squashPathsEqual(ctx context.Context, g gitexec.Git, target, branch string, paths []string) (bool, error) {
	for start := 0; start < len(paths); start += squashPathspecChunk {
		end := min(start+squashPathspecChunk, len(paths))
		args := []string{"-c", diffSubmodulesConfig, "diff", "--quiet", "--no-renames", branch, target, "--"}
		for _, p := range paths[start:end] {
			args = append(args, literalPathspec+p)
		}
		_, stderr, code, err := g.Capture(ctx, args...)
		switch {
		case err != nil || code > 1:
			return false, fmt.Errorf("gc: git diff %s %s: exit %d err=%v: %s", branch, target, code, err, strings.TrimSpace(stderr))
		case code == 1:
			return false, nil
		}
	}
	return true, nil
}
